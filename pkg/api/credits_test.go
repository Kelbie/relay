package api

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
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
			name:    "missing authorization header",
			request: &http.Request{},
			err:     ErrInvalidAuthHeader,
		},
		{
			name:    "invalid auth scheme",
			request: &http.Request{Header: http.Header{"Authorization": []string{"Nastr xxxx"}}},
			err:     ErrInvalidAuthScheme,
		},
		{
			name:    "invalid auth base 64",
			request: &http.Request{Header: http.Header{"Authorization": []string{"Nostr xxx"}}},
			err:     ErrInvalidAuthBase64,
		},
		{
			name:    "invalid auth event",
			request: &http.Request{Header: http.Header{"Authorization": []string{"Nostr " + base64.StdEncoding.EncodeToString([]byte("invalid json"))}}},
			err:     ErrInvalidEventJSON,
		},
		{
			name:    "invalid event kind",
			request: &http.Request{Header: http.Header{"Authorization": []string{"Nostr " + Base64(nostr.Event{})}}},
			err:     ErrInvalidAuthKind,
		},
		{
			name:    "invalid event created_at",
			request: &http.Request{Header: http.Header{"Authorization": []string{"Nostr " + Base64(nostr.Event{Kind: nostr.KindHTTPAuth})}}},
			err:     ErrExpiredAuthEvent,
		},
		{
			name: "invalid event u tag",
			request: &http.Request{
				Header: http.Header{"Authorization": []string{"Nostr " + Base64(
					nostr.Event{
						Kind:      nostr.KindHTTPAuth,
						CreatedAt: nostr.Now(),
						Tags: nostr.Tags{
							{"u", "http://invalid"},
							{"method", "GET"},
						}}),
				}},
				Method: http.MethodGet,
				URL:    &url.URL{Path: "/api/test"},
			},
			err: ErrInvalidAuthURL,
		},
		{
			name: "invalid event method tag",
			request: &http.Request{
				Header: http.Header{"Authorization": []string{"Nostr " + Base64(
					nostr.Event{
						Kind:      nostr.KindHTTPAuth,
						CreatedAt: nostr.Now(),
						Tags: nostr.Tags{
							{"u", "http://example.com/api/test"},
							{"method", "POST"},
						}}),
				}},
				Method: http.MethodGet,
				URL:    &url.URL{Path: "/api/test"},
			},
			err: ErrInvalidAuthMethod,
		},
		{
			name: "valid",
			request: &http.Request{
				Header: http.Header{"Authorization": []string{"Nostr " + Base64(
					Signed(nostr.Event{
						Kind:      nostr.KindHTTPAuth,
						CreatedAt: nostr.Now(),
						Tags: nostr.Tags{
							{"u", "http://example.com/api/test"},
							{"method", "GET"},
						}})),
				}},
				Method: http.MethodGet,
				URL:    &url.URL{Path: "/api/test"},
			},
			pubkey: "3909edd62d1f553df2d2961a9ff61c262387a8bcfe7885b70b3fa56a21f712f0",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := Handler{Domain: "example.com"}
			pubkey, err := h.authNIP98(test.request)
			if !errors.Is(err, test.err) {
				t.Fatalf("expected error %v, got %v", test.err, err)
			}

			if pubkey != test.pubkey {
				t.Fatalf("expected pubkey %v, got %v", test.pubkey, pubkey)
			}
		})
	}
}

func TestParseNIP98(t *testing.T) {
	tests := []struct {
		auth     string
		expected *nostr.Event
		err      error
	}{
		{auth: "", err: ErrInvalidAuthHeader},
		{auth: "Nastr xxx", err: ErrInvalidAuthScheme},
		{auth: "Nostr xxx", err: ErrInvalidAuthBase64},
		{auth: "Nostr Y2lhbw0K", err: ErrInvalidEventJSON},
		{
			auth: "Nostr ew0KICAiaWQiOiAiZmU5NjRlNzU4OTAzMzYwZjI4ZDg0MjRkMDkyZGE4NDk0ZWQyMDdjYmE4MjMxMTBiZTNhNTdkZmU0YjU3ODczNCIsDQogICJwdWJrZXkiOiAiNjNmZTYzMThkYzU4NTgzY2ZlMTY4MTBmODZkZDA5ZTE4YmZkNzZhYWJjMjRhMDA4MWNlMjg1NmYzMzA1MDRlZCIsDQogICJjb250ZW50IjogIiIsDQogICJraW5kIjogMjcyMzUsDQogICJjcmVhdGVkX2F0IjogMTY4MjMyNzg1MiwNCiAgInRhZ3MiOiBbDQogICAgWyJ1IiwgImh0dHBzOi8vYXBpLnNub3J0LnNvY2lhbC9hcGkvdjEvbjVzcC9saXN0Il0sDQogICAgWyJtZXRob2QiLCAiR0VUIl0NCiAgXSwNCiAgInNpZyI6ICI1ZWQ5ZDhlYzk1OGJjODU0Zjk5N2JkYzI0YWMzMzdkMDA1YWYzNzIzMjQ3NDdlZmU0YTAwZTI0ZjRjMzA0MzdmZjRkZDgzMDg2ODRiZWQ0NjdkOWQ2YmUzZTVhNTE3YmI0M2IxNzMyY2M3ZDMzOTQ5YTNhYWY4NjcwNWMyMjE4NCINCn0=",
			expected: &nostr.Event{
				ID:        "fe964e758903360f28d8424d092da8494ed207cba823110be3a57dfe4b578734",
				PubKey:    "63fe6318dc58583cfe16810f86dd09e18bfd76aabc24a0081ce2856f330504ed",
				Kind:      27235,
				CreatedAt: 1682327852,
				Tags: nostr.Tags{
					{"u", "https://api.snort.social/api/v1/n5sp/list"},
					{"method", "GET"},
				},
				Sig: "5ed9d8ec958bc854f997bdc24ac337d005af372324747efe4a00e24f4c30437ff4dd8308684bed467d9d6be3e5a517bb43b1732cc7d33949a3aaf86705c22184",
			},
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("Case=%d", i), func(t *testing.T) {
			event, err := parseNIP98(test.auth)
			if !errors.Is(err, test.err) {
				t.Fatalf("expected error %v, got %v", test.err, err)
			}

			if !reflect.DeepEqual(event, test.expected) {
				t.Fatalf("expected event %v, got %v", test.expected, event)
			}
		})
	}
}

func Base64(e nostr.Event) string {
	bytes, err := e.MarshalJSON()
	if err != nil {
		panic(err)
	}
	return base64.URLEncoding.EncodeToString(bytes)
}

var sk = "6c670052fb1ea99a2b8e03895fea6df717e3c54fef676ab320dc59a57dfea441"

func Signed(e nostr.Event) nostr.Event {
	e.Sign(sk)
	return e
}
