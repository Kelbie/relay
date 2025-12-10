package api

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/goccy/go-json"
	"github.com/nbd-wtf/go-nostr"
)

const (
	authTimeTolerance = time.Minute
)

var (
	ErrInvalidAuthHeader = errors.New("missing or malformed Authorization header")
	ErrInvalidAuthScheme = errors.New("authorization scheme must be 'Nostr <base64_event>'")
	ErrInvalidAuthBase64 = errors.New("failed to decode base64 event payload")
	ErrInvalidAuthKind   = errors.New("kind must be 27235")
	ErrExpiredAuthEvent  = errors.New("created_at is outside the allowed time window")
	ErrInvalidAuthURL    = errors.New("'u' tag does not match request URL")
	ErrInvalidAuthMethod = errors.New("'method' tag does not match request method")
)

// authNIP98 performs all required NIP-98 auth validation (no "payload" check),
// returning the pubkey that performed the authentication, or an error if invalid.
func authNIP98(r *http.Request) (pubkey string, err error) {
	event, err := parseNIP98(r.Header.Get("Authorization"))
	if err != nil {
		return "", err
	}

	if event.Kind != nostr.KindHTTPAuth {
		return "", ErrInvalidAuthKind
	}

	if time.Since(event.CreatedAt.Time()).Abs() > authTimeTolerance {
		return "", ErrExpiredAuthEvent
	}

	if tagValue(event, "method") != r.Method {
		return "", ErrInvalidAuthMethod
	}

	if tagValue(event, "u") != normalizeURL(r) {
		return "", fmt.Errorf("%w: expected %v, got %v", ErrInvalidAuthURL, normalizeURL(r), tagValue(event, "u"))
	}

	if err := verify(event); err != nil {
		return "", err
	}
	return event.PubKey, nil
}

// parseNIP98 from the authentication string.
func parseNIP98(auth string) (*nostr.Event, error) {
	parts := strings.Split(auth, " ")
	if len(parts) != 2 {
		return nil, ErrInvalidAuthHeader
	}

	if parts[0] != "Nostr" {
		return nil, ErrInvalidAuthScheme
	}

	bytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidAuthBase64, err)
	}

	event := &nostr.Event{}
	if err := json.Unmarshal(bytes, event); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidEventJSON, err)
	}
	return event, err
}

func tagValue(e *nostr.Event, tagKey string) string {
	for _, tag := range e.Tags {
		if len(tag) >= 2 && tag[0] == tagKey {
			return tag[1]
		}
	}
	return ""
}

func normalizeURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}

	host := r.Host
	if h := r.Header.Get("X-Forwarded-Host"); h != "" {
		host = h
	}
	return scheme + "://" + host + r.URL.RequestURI()
}
