package dvm

import (
	"context"
	"math"
	"reflect"
	"sort"
	"testing"

	"github.com/nbd-wtf/go-nostr"
	mockdb "github.com/vertex-lab/crawler/pkg/database/mock"
	mockstore "github.com/vertex-lab/crawler/pkg/store/mock"
	"github.com/vertex-lab/relay/pkg/eventstore"
)

const maxDist float64 = 0.002

func TestResponseEvent(t *testing.T) {
	record := Record{ID: "xxx", Kind: KindSortProfiles, Pubkey: fran, CreatedAt: 420}
	tests := []struct {
		name     string
		res      Response
		req      *Request
		expected *nostr.Event
	}{
		{
			name: "empty res",
			res:  Response{},
			req:  &Request{Record: record, Algorithm: Algorithm{Sort: Global}},
			expected: &nostr.Event{
				Content:   "[]",
				CreatedAt: 420,
				Kind:      KindSortProfiles + 1000,
				Tags:      nostr.Tags{{"e", "xxx"}, {"p", fran}, {"sort", Global}},
			},
		},
		{
			name: "response from empty ranking and extras",
			res:  NewResponse(nil),
			req:  &Request{Record: record, Algorithm: Algorithm{Sort: Global}},
			expected: &nostr.Event{
				Content:   "[]",
				CreatedAt: 420,
				Kind:      KindSortProfiles + 1000,
				Tags:      nostr.Tags{{"e", "xxx"}, {"p", fran}, {"sort", Global}},
			},
		},
		{
			name: "valid",
			res: Response{
				{Pubkey: "abc", Rank: 0.1, Extra: Extra{Follows: intPtr(69), Followers: intPtr(420)}},
				{Pubkey: "123", Rank: 0.2},
			},
			req: &Request{Record: record, Algorithm: Algorithm{Sort: Global}},
			expected: &nostr.Event{
				Content:   "[{\"pubkey\":\"abc\",\"rank\":0.1,\"follows\":69,\"followers\":420},{\"pubkey\":\"123\",\"rank\":0.2}]",
				CreatedAt: 420,
				Kind:      KindSortProfiles + 1000,
				Tags:      nostr.Tags{{"e", "xxx"}, {"p", fran}, {"sort", Global}},
			},
		},
		{
			name: "valid personalized",
			res: Response{
				{Pubkey: "abc", Rank: 0.1, Extra: Extra{Follows: intPtr(69), Followers: intPtr(420)}},
				{Pubkey: "123", Rank: 0.2},
			},
			req: &Request{Record: record, Algorithm: Algorithm{Sort: Personalized, Source: pip}},
			expected: &nostr.Event{
				Content:   "[{\"pubkey\":\"abc\",\"rank\":0.1,\"follows\":69,\"followers\":420},{\"pubkey\":\"123\",\"rank\":0.2}]",
				CreatedAt: 420,
				Kind:      KindSortProfiles + 1000,
				Tags:      nostr.Tags{{"e", "xxx"}, {"p", fran}, {"sort", Personalized}, {"source", pip}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event := ResponseEvent(test.res, test.req)
			if !reflect.DeepEqual(event, test.expected) {
				t.Fatalf("ResponseEvent(): expected %v, got %v", test.expected, event)
			}
		})
	}
}

func TestVerifyReputation(t *testing.T) {
	tests := []struct {
		name    string
		DBType  string
		RWSType string
		args    *VerifyReputationArgs

		ranking Ranking
		extras  []Extra
	}{
		{
			name:    "target not in the DB",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &VerifyReputationArgs{
				Algorithm: Algorithm{Sort: Global},
				Target:    randomKey,
				Limit:     5,
			},
			ranking: Ranking{{Key: randomKey, Val: 0}},
		},
		{
			name:    "valid global (simple)",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &VerifyReputationArgs{
				Algorithm: Algorithm{Sort: Global},
				Target:    calle,
				Limit:     1,
			},
			ranking: Ranking{{Key: calle, Val: 0.5}, {Key: odell, Val: 0.5}},
			extras:  []Extra{{Follows: intPtr(0), Followers: intPtr(1)}},
		},
		{
			name:    "valid global (triangle)",
			DBType:  "triangle",
			RWSType: "triangle",
			args: &VerifyReputationArgs{
				Algorithm: Algorithm{Sort: Global},
				Target:    "2",
				Limit:     1,
			},
			ranking: Ranking{{Key: "2", Val: 0.33333}, {Key: "1", Val: 0.33333}},
			extras:  []Extra{{Follows: intPtr(1), Followers: intPtr(1)}},
		},
		{
			name:    "valid personalized (simple)",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &VerifyReputationArgs{
				Algorithm: Algorithm{Sort: Personalized, Source: odell},
				Target:    calle,
				Limit:     1,
			},
			ranking: Ranking{{Key: calle, Val: 0.45946}, {Key: odell, Val: 0.54054}},
			extras:  []Extra{{Follows: intPtr(0), Followers: intPtr(1)}},
		},
		{
			name:    "valid personalized (triangle)",
			DBType:  "triangle",
			RWSType: "triangle",
			args: &VerifyReputationArgs{
				Algorithm: Algorithm{Sort: Personalized, Source: "0"},
				Target:    "2",
				Limit:     1,
			},
			ranking: Ranking{{Key: "2", Val: 0.280855199}, {Key: "1", Val: 0.330417881}},
			extras:  []Extra{{Follows: intPtr(1), Followers: intPtr(1)}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			DB := mockdb.SetupDB(test.DBType)
			RWS := mockstore.SetupRWS(test.RWSType)

			ranking, extras, err := verifyReputation(ctx, DB, RWS, test.args)
			if err != nil {
				t.Fatalf("expected error nil, got %v", err)
			}

			dist := distance(ranking, test.ranking)
			if dist > maxDist {
				t.Errorf("VerifyReputation: expected distance %v, got %v", maxDist, dist)
				t.Errorf("expected ranking %v, got %v", test.ranking, ranking)
			}

			if !reflect.DeepEqual(extras, test.extras) {
				t.Errorf("expected extras %v, got %v", test.extras, extras)
			}
		})
	}
}

func TestSortProfiles(t *testing.T) {
	tests := []struct {
		name     string
		DBType   string
		RWSType  string
		args     *SortProfilesArgs
		expected Ranking
	}{
		{
			name:    "valid global (one target not found in the DB)",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &SortProfilesArgs{
				Algorithm: Algorithm{Sort: Global},
				Targets:   []string{randomKey, calle, pip},
				Limit:     3,
			},
			expected: Ranking{
				{Key: calle, Val: 0.5},
				{Key: pip, Val: 0.0},
				{Key: randomKey, Val: 0.0},
			},
		},
		{
			name:    "valid global (triangle)",
			DBType:  "triangle",
			RWSType: "triangle",
			args: &SortProfilesArgs{
				Algorithm: Algorithm{Sort: Global},
				Targets:   []string{"0", "1", "2", "69"},
				Limit:     4,
			},
			expected: Ranking{
				{Key: "0", Val: 0.33333},
				{Key: "1", Val: 0.33333},
				{Key: "2", Val: 0.33333},
				{Key: "69", Val: 0},
			},
		},
		{
			name:    "valid personalized (simple)",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &SortProfilesArgs{
				Algorithm: Algorithm{Sort: Personalized, Source: odell},
				Targets:   []string{odell, calle, pip},
				Limit:     3,
			},
			expected: Ranking{
				{Key: odell, Val: 0.540540541},
				{Key: calle, Val: 0.459459459},
				{Key: pip, Val: 0.0},
			},
		},
		{
			name:    "valid personalized (triangle)",
			DBType:  "triangle",
			RWSType: "triangle",
			args: &SortProfilesArgs{
				Algorithm: Algorithm{Sort: Personalized, Source: "0"},
				Targets:   []string{"0", "1", "2"},
				Limit:     3,
			},
			expected: Ranking{
				{Key: "0", Val: 0.388726919},
				{Key: "1", Val: 0.330417881},
				{Key: "2", Val: 0.280855199},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			DB := mockdb.SetupDB(test.DBType)
			RWS := mockstore.SetupRWS(test.RWSType)

			ranking, err := sortProfiles(ctx, DB, RWS, test.args)
			if err != nil {
				t.Fatalf("expected error nil, got %v", err)
			}

			dist := distance(ranking, test.expected)
			if dist > maxDist {
				t.Errorf("VerifyReputation: expected distance %v, got %v", maxDist, dist)
				t.Errorf("expected ranking %v, got %v", test.expected, ranking)
			}
		})
	}
}

func TestSearchAuthors(t *testing.T) {
	tests := []struct {
		name     string
		DBType   string
		RWSType  string
		args     *SearchProfilesArgs
		expected Ranking
	}{
		{
			name:    "valid global",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &SearchProfilesArgs{
				Algorithm: Algorithm{Sort: Global},
				Search:    "pip",
				Limit:     5,
			},
			expected: Ranking{{Key: pip, Val: 0.0}},
		},
		{
			name:    "valid global npub",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &SearchProfilesArgs{
				Algorithm: Algorithm{Sort: Global},
				Search:    "npub176p7sup477k5738qhxx0hk2n0cty2k5je5uvalzvkvwmw4tltmeqw7vgup",
				Limit:     5,
			},
			expected: Ranking{{Key: pip, Val: 0.0}},
		},
		{
			name:    "valid global hex",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &SearchProfilesArgs{
				Algorithm: Algorithm{Sort: Global},
				Search:    pip,
				Limit:     5,
			},
			expected: Ranking{{Key: pip, Val: 0.0}},
		},
		{
			name:    "valid no results",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &SearchProfilesArgs{
				Algorithm: Algorithm{Sort: Global},
				Search:    "jack",
				Limit:     5,
			},
		},
		{
			name:    "valid personalized (simple)",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &SearchProfilesArgs{
				Algorithm: Algorithm{Sort: Personalized, Source: odell},
				Search:    "pip",
				Limit:     5,
			},
			expected: Ranking{{Key: pip, Val: 0.0}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			DB := mockdb.SetupDB(test.DBType)
			RWS := mockstore.SetupRWS(test.RWSType)
			eventStore, err := eventstore.New("test.sqlite")
			if err != nil {
				t.Fatal(err)
			}

			ranking, err := searchProfiles(ctx, DB, RWS, eventStore, test.args)
			if err != nil {
				t.Fatalf("expected error nil, got %v", err)
			}

			dist := distance(ranking, test.expected)
			if dist > maxDist {
				t.Errorf("VerifyReputation: expected distance %v, got %v", maxDist, dist)
				t.Errorf("expected ranking %v, got %v", test.expected, ranking)
			}
		})
	}
}

func TestRecommendFollows(t *testing.T) {
	tests := []struct {
		name     string
		DBType   string
		RWSType  string
		args     *RecommendFollowsArgs
		expected Ranking
	}{
		{
			name:    "valid global (simple)",
			DBType:  "simple-with-pks",
			RWSType: "simple",
			args: &RecommendFollowsArgs{
				Algorithm: Algorithm{Sort: Global, Source: randomKey},
				Limit:     2,
			},
			expected: Ranking{
				{Key: calle, Val: 0.5},
				{Key: odell, Val: 0.5},
			},
		},
		{
			name:    "valid global (triangle)",
			DBType:  "triangle",
			RWSType: "triangle",
			args: &RecommendFollowsArgs{
				Algorithm: Algorithm{Sort: Global, Source: "0"},
				Limit:     1,
			},
			expected: Ranking{{Key: "2", Val: 1.0 / 3.0}},
		},
		{
			name:    "valid personalized",
			DBType:  "triangle",
			RWSType: "triangle",
			args: &RecommendFollowsArgs{
				Algorithm: Algorithm{Sort: Personalized, Source: "0"},
				Limit:     1,
			},
			expected: Ranking{{Key: "2", Val: 0.2809}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			DB := mockdb.SetupDB(test.DBType)
			RWS := mockstore.SetupRWS(test.RWSType)

			ranking, err := recommendFollows(ctx, DB, RWS, test.args)
			if err != nil {
				t.Fatalf("expected error nil, got %v", err)
			}

			dist := distance(ranking, test.expected)
			if dist > maxDist {
				t.Errorf("VerifyReputation: expected distance %v, got %v", maxDist, dist)
				t.Errorf("expected ranking %v, got %v", test.expected, ranking)
			}
		})
	}
}

// -----------------------------------HELPERS-----------------------------------

// distance() returns the L1 distance between two Rankings.
func distance(r1, r2 Ranking) float64 {
	if len(r1) != len(r2) {
		return math.MaxFloat64
	}

	// sort the responses in lexicographic order of the keys before comparing
	sort.Slice(r1, func(i, j int) bool { return r1[i].Key > r1[j].Key })
	sort.Slice(r2, func(i, j int) bool { return r2[i].Key > r2[j].Key })

	var distance float64
	for i := range r1 {
		if r1[i].Key != r2[i].Key {
			// if the keys are different, the two ranking are incomparable
			return math.MaxFloat64
		}

		distance += math.Abs(r1[i].Val - r2[i].Val)
	}

	return distance
}

func intPtr(i int) *int {
	return &i
}
