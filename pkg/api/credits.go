package api

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/goccy/go-json"

	"github.com/nbd-wtf/go-nostr"
)

const (
	authTimeTolerance = time.Minute
)

var (
	ErrInvalidAuthHeader = errors.New("missing or malformed Authorization header")
	ErrInvalidAuthScheme = errors.New("authorization scheme must be 'Nostr'")
	ErrInvalidAuthBase64 = errors.New("failed to decode base64 event payload")
	ErrInvalidAuthKind   = errors.New("kind must be 27235")
	ErrExpiredAuthEvent  = errors.New("created_at is outside the allowed time window")
	ErrInvalidAuthURL    = errors.New("'u' tag does not match request URL")
	ErrInvalidAuthMethod = errors.New("'method' tag does not match request method")
)

// HandleCredits handles the endpoint GET /api/v1/credits
func (h Handler) GetCredits(w http.ResponseWriter, r *http.Request) {

}

// authNIP98 performs all required NIP-98 auth validation (no "payload" check),
// returning the pubkey that performed the authentication, or an error if invalid.
func authNIP98(r *http.Request) (pubkey string, err error) {
	event, err := parseNIP98(r.Header)
	if err != nil {
		return "", err
	}

	if event.Kind != nostr.KindHTTPAuth {
		return "", ErrInvalidAuthKind
	}

	if time.Since(event.CreatedAt.Time()).Abs() > authTimeTolerance {
		return "", ErrExpiredAuthEvent
	}

	if tagValue(event, "u") != r.URL.String() {
		return "", ErrInvalidAuthURL
	}

	if tagValue(event, "method") != r.Method {
		return "", ErrInvalidAuthMethod
	}

	if err := verify(event); err != nil {
		return "", err
	}
	return event.PubKey, nil
}

func parseNIP98(header http.Header) (*nostr.Event, error) {
	auth := header.Values("Authorization")
	if len(auth) != 2 {
		return nil, ErrInvalidAuthHeader
	}

	if auth[0] != "Nostr" {
		return nil, ErrInvalidAuthScheme
	}

	eventJsonBytes, err := base64.StdEncoding.DecodeString(auth[1])
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidAuthBase64, err)
	}

	event := &nostr.Event{}
	if err := json.Unmarshal(eventJsonBytes, event); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidEventJSON, err)
	}
	return event, err
}

func requiresBody(method string) bool {
	return method == http.MethodPost || method == http.MethodPatch || method == http.MethodPut
}

func tagValue(e *nostr.Event, tagKey string) string {
	for _, tag := range e.Tags {
		if len(tag) >= 2 && tag[0] == tagKey {
			return tag[1]
		}
	}
	return ""
}
