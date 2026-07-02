package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/vertex-lab/relay/pkg/api"
	"github.com/vertex-lab/relay/pkg/config"
	"github.com/vertex-lab/relay/pkg/core"
	openranking "github.com/vertex-lab/relay/pkg/open-ranking"
	"github.com/vertex-lab/relay/pkg/ranking"
	"github.com/vertex-lab/relay/pkg/rate"
	"github.com/vertex-lab/relay/pkg/relay"

	"github.com/nbd-wtf/go-nostr"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// removing go-nostr logs
	nostr.DebugLogger.SetOutput(io.Discard)
	nostr.InfoLogger.SetOutput(io.Discard)

	slog.Info("--------- starting up the relay --------")
	defer slog.Info("-----------------------------------------")

	config, err := config.Load()
	if err != nil {
		panic(err)
	}

	core, err := core.NewService(config.Core)
	if err != nil {
		panic(err)
	}
	defer core.Close()

	limiter := rate.NewLimiter(config.Limiter)
	api := api.NewHandler(config.API, core, limiter)
	relay := relay.Setup(config.Relay, core, limiter)

	router := http.NewServeMux()
	router.HandleFunc("POST /api/v1/dvms", api.HandleDVMs)
	router.HandleFunc("GET /api/v1/credits", api.GetCredits)
	router.Handle("/", relay)

	legacyServer := http.Server{
		Addr:              config.Relay.Address,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	service, err := ranking.NewService(config.Service)
	if err != nil {
		panic(err)
	}
	defer service.Close()
	server := openranking.NewServer(config.OpenRanking, service, limiter)

	exit := make(chan error, 2)
	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		relay.Start(ctx)
		err := legacyServer.ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) {
			exit <- err
		}
	}()

	go func() {
		defer wg.Done()
		err := server.StartAndServe(ctx, config.OpenRanking.Address)
		if !errors.Is(err, http.ErrServerClosed) {
			exit <- err
		}
	}()

	select {
	case <-ctx.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := legacyServer.Shutdown(ctx)
		relay.Wait()
		wg.Wait()
		if err != nil {
			panic(err)
		}

	case err := <-exit:
		panic(err)
	}
}
