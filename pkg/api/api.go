package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/goccy/go-json"
	"github.com/nbd-wtf/go-nostr"
	"github.com/pippellia-btc/rely"
	"github.com/vertex-lab/relay/pkg/core"
	"github.com/vertex-lab/relay/pkg/dvm"
	"github.com/vertex-lab/relay/pkg/rate"
)

const MaxRequestBody = 500_000 // 0.5MB

var (
	ErrInvalidEventJSON = errors.New("invalid event json")
)

type Handler struct {
	service   *core.Service
	limiter   *rate.Limiter
	secretKey string

	stats
}

// GetCredits handles the endpoint GET /api/v1/credits
func (h *Handler) GetCredits(w http.ResponseWriter, r *http.Request) {
	ip := rely.GetIP(r).Group()
	if h.limiter.Reject(ip, 1) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte("Rate limit exceeded. Try again later."))
		return
	}

	pubkey, err := authNIP98(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	bucket, err := h.service.Credits.Bucket(pubkey)
	if err != nil {
		http.Error(w, "internal error while retrieving the credits", http.StatusInternalServerError)
		return
	}

	credits := bucket.ToEvent()
	if err := credits.Sign(h.secretKey); err != nil {
		// the handler failed to sign the response, likely caused by an invalid secret key.
		// This is an unrecoverable error since all responses must be signed.
		panic(fmt.Errorf("api.Handler.GetCredits: failed to sign: %w", err))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(credits)
	if err != nil {
		slog.Error("encoding failed", "error", err)
		return
	}

	h.stats.Record(statsCredit)
}

// HandleDVMs handles the endpoint /api/v1/dvms
func (h *Handler) HandleDVMs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed. use POST", http.StatusBadRequest)
		return
	}

	ip := rely.GetIP(r).Group()
	if h.limiter.Reject(ip, 1) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte("Rate limit exceeded. Try again later."))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBody)
	event, err := ParseDVM(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	response := dvm.Handler{Service: h.service, SecretKey: h.secretKey}.Process(ctx, event)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		slog.Error("encoding failed", "error", err)
		return
	}

	h.stats.Record(statsDVM)
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

	if err := verify(event); err != nil {
		return nil, err
	}
	return event, nil
}

// Verify returns an error if the event has invalid ID or signature, nil otherwise.
func verify(e *nostr.Event) error {
	if !e.CheckID() {
		return rely.ErrInvalidEventID
	}

	match, err := e.CheckSignature()
	if err != nil {
		return fmt.Errorf("%w: %w", rely.ErrInvalidEventSignature, err)
	}
	if !match {
		return rely.ErrInvalidEventSignature
	}
	return nil
}
