package ranking

import (
	"fmt"
	"strings"
	"testing"
)

const (
	validPubkey  = "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d"
	validPubkey2 = "82341f882b6eabcd2ba7f1ef90aad961cf074af15b9ef44a09f9d2a8fbfbe6a2"
)

func TestCompromisedPubkeysRequest_Normalize(t *testing.T) {
	tests := []struct {
		name    string
		req     CompromisedPubkeysRequest
		wantErr bool
		algo    string
	}{
		{"defaults algo", CompromisedPubkeysRequest{Pubkeys: []string{validPubkey}}, false, string(SignatureProof)},
		{"explicit algo", CompromisedPubkeysRequest{Pubkeys: []string{validPubkey}, Algorithm: SignatureProof}, false, string(SignatureProof)},
		{"deduplicates", CompromisedPubkeysRequest{Pubkeys: []string{validPubkey, validPubkey}}, false, string(SignatureProof)},
		{"empty pubkeys", CompromisedPubkeysRequest{}, true, ""},
		{"too many pubkeys", CompromisedPubkeysRequest{Pubkeys: pubkeys(101)}, true, ""},
		{"invalid algo", CompromisedPubkeysRequest{Pubkeys: []string{validPubkey}, Algorithm: "bad"}, true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Normalize()
			if (err != nil) != tt.wantErr {
				t.Fatalf("got err=%v, wantErr=%v", err, tt.wantErr)
			}
			if !tt.wantErr && string(tt.req.Algorithm) != tt.algo {
				t.Fatalf("algo=%s, want %s", tt.req.Algorithm, tt.algo)
			}
		})
	}
}

func TestFollowersRequest_Normalize(t *testing.T) {
	tests := []struct {
		name      string
		req       FollowersRequest
		wantErr   bool
		wantLimit int
	}{
		{"defaults", FollowersRequest{Pubkey: validPubkey}, false, 50},
		{"explicit limit", FollowersRequest{Pubkey: validPubkey, Limit: 10}, false, 10},
		{"limit too high", FollowersRequest{Pubkey: validPubkey, Limit: 1001}, true, 0},
		{"negative limit", FollowersRequest{Pubkey: validPubkey, Limit: -1}, true, 0},
		{"empty pubkey", FollowersRequest{}, true, 0},
		{"invalid pubkey", FollowersRequest{Pubkey: "bad"}, true, 0},
		{"invalid algo", FollowersRequest{Pubkey: validPubkey, Algorithm: "bad"}, true, 0},
		{"ppr requires pov", FollowersRequest{Pubkey: validPubkey, Algorithm: PersonalizedPagerank}, true, 0},
		{"ppr with valid pov", FollowersRequest{Pubkey: validPubkey, Algorithm: PersonalizedPagerank, POV: validPubkey2}, false, 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Normalize()
			if (err != nil) != tt.wantErr {
				t.Fatalf("got err=%v, wantErr=%v", err, tt.wantErr)
			}
			if !tt.wantErr && tt.req.Limit != tt.wantLimit {
				t.Fatalf("limit=%d, want %d", tt.req.Limit, tt.wantLimit)
			}
		})
	}
}

func TestRankPubkeysRequest_Normalize(t *testing.T) {
	two := []string{validPubkey, validPubkey2}
	tests := []struct {
		name      string
		req       RankPubkeysRequest
		wantErr   bool
		wantLimit int
	}{
		{"defaults limit to len", RankPubkeysRequest{Pubkeys: two}, false, 2},
		{"deduplicates", RankPubkeysRequest{Pubkeys: []string{validPubkey, validPubkey}}, false, 1},
		{"explicit limit", RankPubkeysRequest{Pubkeys: two, Limit: 1}, false, 1},
		{"limit capped to len", RankPubkeysRequest{Pubkeys: two, Limit: 100}, false, 2},
		{"negative limit", RankPubkeysRequest{Pubkeys: two, Limit: -1}, true, 0},
		{"empty pubkeys", RankPubkeysRequest{}, true, 0},
		{"invalid algo", RankPubkeysRequest{Pubkeys: two, Algorithm: "bad"}, true, 0},
		{"ppr requires pov", RankPubkeysRequest{Pubkeys: two, Algorithm: PersonalizedPagerank}, true, 0},
		{"ppr with valid pov", RankPubkeysRequest{Pubkeys: two, Algorithm: PersonalizedPagerank, POV: validPubkey}, false, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Normalize()
			if (err != nil) != tt.wantErr {
				t.Fatalf("got err=%v, wantErr=%v", err, tt.wantErr)
			}
			if !tt.wantErr && tt.req.Limit != tt.wantLimit {
				t.Fatalf("limit=%d, want %d", tt.req.Limit, tt.wantLimit)
			}
		})
	}
}

func TestRecommendPubkeysRequest_Normalize(t *testing.T) {
	tests := []struct {
		name      string
		req       RecommendPubkeysRequest
		wantErr   bool
		wantLimit int
	}{
		{"defaults", RecommendPubkeysRequest{}, false, 20},
		{"explicit limit", RecommendPubkeysRequest{Limit: 5}, false, 5},
		{"limit too high", RecommendPubkeysRequest{Limit: 101}, true, 0},
		{"negative limit", RecommendPubkeysRequest{Limit: -1}, true, 0},
		{"invalid algo", RecommendPubkeysRequest{Algorithm: "bad"}, true, 0},
		{"ppr requires pov", RecommendPubkeysRequest{Algorithm: PersonalizedPagerank}, true, 0},
		{"ppr with valid pov", RecommendPubkeysRequest{Algorithm: PersonalizedPagerank, POV: validPubkey}, false, 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Normalize()
			if (err != nil) != tt.wantErr {
				t.Fatalf("got err=%v, wantErr=%v", err, tt.wantErr)
			}
			if !tt.wantErr && tt.req.Limit != tt.wantLimit {
				t.Fatalf("limit=%d, want %d", tt.req.Limit, tt.wantLimit)
			}
		})
	}
}

func TestSearchPubkeysRequest_Normalize(t *testing.T) {
	tests := []struct {
		name      string
		req       SearchPubkeysRequest
		wantErr   bool
		wantLimit int
	}{
		{"defaults", SearchPubkeysRequest{Query: "alice"}, false, 10},
		{"explicit limit", SearchPubkeysRequest{Query: "alice", Limit: 5}, false, 5},
		{"limit too high", SearchPubkeysRequest{Query: "alice", Limit: 101}, true, 0},
		{"negative limit", SearchPubkeysRequest{Query: "alice", Limit: -1}, true, 0},
		{"query too short", SearchPubkeysRequest{Query: "ab"}, true, 0},
		{"query too long", SearchPubkeysRequest{Query: strings.Repeat("a", 101)}, true, 0},
		{"query trimmed ok", SearchPubkeysRequest{Query: "  alice  "}, false, 10},
		{"invalid algo", SearchPubkeysRequest{Query: "alice", Algorithm: "bad"}, true, 0},
		{"ppr requires pov", SearchPubkeysRequest{Query: "alice", Algorithm: PersonalizedPagerank}, true, 0},
		{"ppr with valid pov", SearchPubkeysRequest{Query: "alice", Algorithm: PersonalizedPagerank, POV: validPubkey}, false, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Normalize()
			if (err != nil) != tt.wantErr {
				t.Fatalf("got err=%v, wantErr=%v", err, tt.wantErr)
			}
			if !tt.wantErr && tt.req.Limit != tt.wantLimit {
				t.Fatalf("limit=%d, want %d", tt.req.Limit, tt.wantLimit)
			}
		})
	}
}

func TestStatsPubkeyRequest_Normalize(t *testing.T) {
	tests := []struct {
		name    string
		req     StatsPubkeyRequest
		wantErr bool
		algo    string
	}{
		{"defaults", StatsPubkeyRequest{Pubkey: validPubkey}, false, string(GlobalPagerank)},
		{"explicit algo", StatsPubkeyRequest{Pubkey: validPubkey, Algorithm: FollowersCount}, false, string(FollowersCount)},
		{"empty pubkey", StatsPubkeyRequest{}, true, ""},
		{"invalid pubkey", StatsPubkeyRequest{Pubkey: "bad"}, true, ""},
		{"invalid algo", StatsPubkeyRequest{Pubkey: validPubkey, Algorithm: "bad"}, true, ""},
		{"ppr requires pov", StatsPubkeyRequest{Pubkey: validPubkey, Algorithm: PersonalizedPagerank}, true, ""},
		{"ppr with valid pov", StatsPubkeyRequest{Pubkey: validPubkey, Algorithm: PersonalizedPagerank, POV: validPubkey2}, false, string(PersonalizedPagerank)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Normalize()
			if (err != nil) != tt.wantErr {
				t.Fatalf("got err=%v, wantErr=%v", err, tt.wantErr)
			}
			if !tt.wantErr && string(tt.req.Algorithm) != tt.algo {
				t.Fatalf("algo=%s, want %s", tt.req.Algorithm, tt.algo)
			}
		})
	}
}

// helpers

func pubkeys(n int) []string {
	s := make([]string, n)
	for i := range s {
		// hex string with unique suffix, not real pubkeys, only length matters
		s[i] = fmt.Sprintf("%063x%x", i, i)
	}
	return s
}
