package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/vertex-lab/relay/pkg/api"
	"github.com/vertex-lab/relay/pkg/config"
	"github.com/vertex-lab/relay/pkg/core"
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

	service, err := core.NewService(config.Service)
	if err != nil {
		panic(err)
	}
	defer service.Close()

	limiter := rate.NewLimiter(config.Limiter)
	api := api.NewHandler(config.API, service, limiter)
	relay := relay.Setup(config.Relay, service, limiter)

	router := http.NewServeMux()
	router.HandleFunc("POST /api/v1/dvms", api.HandleDVMs)
	router.HandleFunc("GET /api/v1/credits", api.GetCredits)
	router.Handle("/", relay)

	exit := make(chan error, 1)
	server := http.Server{
		Addr:              config.Relay.Address,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		relay.Start(ctx)
		err := server.ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) {
			exit <- err
		}
	}()

	select {
	case <-ctx.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := server.Shutdown(ctx)
		relay.Wait()
		if err != nil {
			panic(err)
		}

	case err := <-exit:
		panic(err)
	}
}
