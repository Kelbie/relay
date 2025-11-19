package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/goccy/go-json"
	"github.com/vertex-lab/relay/pkg/core"
	"github.com/vertex-lab/relay/pkg/dvm"

	"github.com/nbd-wtf/go-nostr"
	"github.com/pippellia-btc/rely"
)

var ErrInvalidEventJSON = errors.New("invalid event json")

type Handler struct {
	Service   *core.Service
	SecretKey string
}

// HandleDVMs handles the endpoint /api/v1/dvms
func (h Handler) HandleDVMs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed. use POST", http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	event, err := ParseDVM(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	handler := dvm.Handler{
		Service:   h.Service,
		SecretKey: h.SecretKey,
	}
	response := handler.Process(ctx, event)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		slog.Error("encoding failed", "error", err)
	}
}

// ParseDVM parses the nostr event from the request body.
// It returns an error if the event is malformed, has an invalid id or signature.
func ParseDVM(r *http.Request) (*nostr.Event, error) {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	event := &nostr.Event{}
	if err := decoder.Decode(&event); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidEventJSON, err)
	}

	if !event.CheckID() {
		return nil, rely.ErrInvalidEventID
	}

	match, err := event.CheckSignature()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", rely.ErrInvalidEventSignature, err)
	}
	if !match {
		return nil, rely.ErrInvalidEventSignature
	}
	return event, nil
}
