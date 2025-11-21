package api

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/nbd-wtf/go-nostr"
)

var (
	validAuth      = []byte(`{"kind":27235,"id":"c67244e7c080266b4f76ee00b381e92d3b24b502a05b74436d1793abc80423a3","pubkey":"79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798","created_at":1763746871,"tags":[["u","https://example.com/api/test"],["method","GET"]],"content":"","sig":"9dda1cb882ee998adc331aa9333033655d818fcf5ab3b82bad02eb69cfe02bbc49b8b0b98d45f24840a483f476633befe3daa951644bc1e0624c41a2cf65b384"}`)
	invalidAuthID  = []byte(`{"kind":27235,"id":"c67244e--------invalid-------92d3b24b502a05b74436d1793abc80423a3","pubkey":"79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798","created_at":1763746871,"tags":[["u","https://example.com/api/test"],["method","GET"]],"content":"","sig":"9dda1cb882ee998adc331aa9333033655d818fcf5ab3b82bad02eb69cfe02bbc49b8b0b98d45f24840a483f476633befe3daa951644bc1e0624c41a2cf65b384"}`)
	invalidAuthSig = []byte(`{"kind":27235,"id":"c67244e7c080266b4f76ee00b381e92d3b24b502a05b74436d1793abc80423a3","pubkey":"79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798","created_at":1763746871,"tags":[["u","https://example.com/api/test"],["method","GET"]],"content":"","sig":"9dda1cb882ee998adc331aa9333033655d818fcf5ab3b82bad02eb69cfe02bbc49b8b0b98d45f24840---invalid----951644bc1e0624c41a2cf65b384"}`)
	validEventAuth = []byte(`{"kind":27235,"id":"c67244e7c080266b4f76ee00b381e92d3b24b502a05b74436d1793abc80423a3","pubkey":"79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798","created_at":1763746871,"tags":[["u","https://example.com/api/test"],["method","GET"]],"content":"","sig":"9dda1cb882ee998adc331aa9333033655d818fcf5ab3b82bad02eb69cfe02bbc49b8b0b98d45f24840a483f476633befe3daa951644bc1e0624c41a2cf65b384"}`)
)

func TestAuthNIP98(t *testing.T) {
	tests := []struct {
		name    string
		request *http.Request
		pubkey  string
		err     error
	}{
		{
			name:    "missing authorization",
			request: &http.Request{},
			err:     ErrInvalidAuthHeader,
		},
		{
			name:    "invalid auth scheme",
			request: &http.Request{Header: http.Header{"Authorization": []string{"Nastr", "xxx"}}},
			err:     ErrInvalidAuthScheme,
		},
		{
			name:    "invalid auth base 64",
			request: &http.Request{Header: http.Header{"Authorization": []string{"Nostr", "xxx"}}},
			err:     ErrInvalidAuthBase64,
		},
		{
			name:    "invalid auth event",
			request: &http.Request{Header: http.Header{"Authorization": []string{"Nostr", base64.StdEncoding.EncodeToString([]byte("invalid json"))}}},
			err:     ErrInvalidEventJSON,
		},
		{
			name:    "invalid event kind",
			request: &http.Request{Header: http.Header{"Authorization": []string{"Nostr", Base64(nostr.Event{})}}},
			err:     ErrInvalidAuthKind,
		},
		{
			name:    "invalid event created_at",
			request: &http.Request{Header: http.Header{"Authorization": []string{"Nostr", Base64(nostr.Event{Kind: nostr.KindHTTPAuth})}}},
			err:     ErrExpiredAuthEvent,
		},
		{
			name: "invalid event u tag",
			request: &http.Request{
				Header: http.Header{"Authorization": []string{"Nostr", Base64(
					nostr.Event{
						Kind:      nostr.KindHTTPAuth,
						CreatedAt: nostr.Now(),
						Tags: nostr.Tags{
							{"u", "https://example.com/api/test"},
							{"method", "GET"},
						}}),
				}},
				Method: http.MethodGet,
				URL: &url.URL{
					Scheme: "https",
					Host:   "example.com",
					Path:   "/test1",
				},
			},
			err: ErrInvalidAuthURL,
		},
		{
			name: "invalid event method tag",
			request: &http.Request{
				Header: http.Header{"Authorization": []string{"Nostr", Base64(
					nostr.Event{
						Kind:      nostr.KindHTTPAuth,
						CreatedAt: nostr.Now(),
						Tags: nostr.Tags{
							{"u", "https://example.com/api/test"},
							{"method", "GET"},
						}}),
				}},
				Method: http.MethodPost,
				URL: &url.URL{
					Scheme: "https",
					Host:   "example.com",
					Path:   "/api/test",
				},
			},
			err: ErrInvalidAuthMethod,
		},
		{
			name: "valid",
			request: &http.Request{
				Header: http.Header{"Authorization": []string{"Nostr", Base64(
					Signed(nostr.Event{
						Kind:      nostr.KindHTTPAuth,
						CreatedAt: nostr.Now(),
						Tags: nostr.Tags{
							{"u", "https://example.com/api/test"},
							{"method", "GET"},
						}})),
				}},
				Method: http.MethodGet,
				URL: &url.URL{
					Scheme: "https",
					Host:   "example.com",
					Path:   "/api/test",
				},
			},
			pubkey: "3909edd62d1f553df2d2961a9ff61c262387a8bcfe7885b70b3fa56a21f712f0",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pubkey, err := authNIP98(test.request)
			if !errors.Is(err, test.err) {
				t.Fatalf("expected error %v, got %v", test.err, err)
			}

			if pubkey != test.pubkey {
				t.Fatalf("expected pubkey %v, got %v", test.pubkey, pubkey)
			}
		})
	}
}

func Base64(e nostr.Event) string {
	bytes, err := e.MarshalJSON()
	if err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(bytes)
}

var sk = "6c670052fb1ea99a2b8e03895fea6df717e3c54fef676ab320dc59a57dfea441"

func Signed(e nostr.Event) nostr.Event {
	e.Sign(sk)
	return e
}
