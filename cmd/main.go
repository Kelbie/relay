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
	cfg "github.com/vertex-lab/relay/pkg/config"
	"github.com/vertex-lab/relay/pkg/core"

	"github.com/nbd-wtf/go-nostr"
)

var (
	config  cfg.Config
	service *core.Service
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// removing go-nostr logs
	nostr.DebugLogger.SetOutput(io.Discard)
	nostr.InfoLogger.SetOutput(io.Discard)

	slog.Info("--------- starting up the relay --------")
	defer slog.Info("-----------------------------------------")

	var err error
	config, err = cfg.Load()
	if err != nil {
		panic(err)
	}

	service, err = core.NewService(config.Service)
	if err != nil {
		panic(err)
	}
	defer service.Close()

	api := api.Handler{Service: service, SecretKey: config.Relay.SecretKey, Domain: config.Relay.Domain}
	relay := SetupRelay()

	router := http.NewServeMux()
	router.HandleFunc("POST /api/v1/dvms", api.HandleDVMs)
	router.HandleFunc("GET /api/v1/credits", api.GetCredits)
	router.Handle("/", relay)

	server := http.Server{Addr: config.Relay.Address, Handler: router}
	exitErr := make(chan error, 1)

	go func() {
		relay.Start(ctx)
		err := server.ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) {
			exitErr <- err
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

	case err := <-exitErr:
		panic(err)
	}
}
