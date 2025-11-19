package main

import (
	"context"
	"io"
	"log/slog"
	"os/signal"
	"syscall"

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

	relay := SetupRelay(config.Relay)
	err = relay.StartAndServe(ctx, config.Relay.Address)
	if err != nil {
		panic(err)
	}
}
