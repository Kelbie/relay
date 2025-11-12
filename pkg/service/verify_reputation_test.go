package service

import (
	"errors"
	"fmt"
	"slices"
	"testing"
)

func TestMarshalJSON(t *testing.T) {
	tests := []struct {
		response VerifyReputationResponse
		expected []byte
		err      error
	}{
		{
			response: VerifyReputationResponse{
				target: targetResponse{
					Pubkey:    "ciao",
					Rank:      0.212,
					Follows:   69,
					Followers: 420,
				},
			},

			expected: []byte(`[{"pubkey":"ciao","rank":0.212,"follows":69,"followers":420}]`),
		},
		{
			response: VerifyReputationResponse{
				target: targetResponse{
					Pubkey:    "ciao",
					Rank:      0.212,
					Follows:   69,
					Followers: 420,
				},
				followers: []followerResponse{
					{Pubkey: "pk1", Rank: 0.01},
					{Pubkey: "pk2", Rank: 0.02},
					{Pubkey: "pk3", Rank: 0.03},
					{Pubkey: "pk4", Rank: 0.04},
				},
			},

			expected: []byte(`[{"pubkey":"ciao","rank":0.212,"follows":69,"followers":420},{"pubkey":"pk1","rank":0.01},{"pubkey":"pk2","rank":0.02},{"pubkey":"pk3","rank":0.03},{"pubkey":"pk4","rank":0.04}]`),
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("case=%d", i), func(t *testing.T) {
			json, err := test.response.MarshalJSON()
			if !errors.Is(err, test.err) {
				t.Fatalf("expected error %v, got %v", test.err, err)
			}

			if !slices.Equal(json, test.expected) {
				t.Fatalf("expected json %v, got %v", string(test.expected), string(json))
			}
		})
	}
}
