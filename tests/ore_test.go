package tests

import (
	"context"
	"net/http"
	"slices"
	"testing"
	"time"

	ore "github.com/Open-Ranking/go-sdk"
)

var (
	oreEndpoint = "http://localhost:8080"
	oreClient   *ore.Client
)

func init() {
	var err error
	oreClient, err = ore.NewClient(oreEndpoint, &http.Client{Timeout: 5 * time.Second})
	if err != nil {
		panic(err)
	}
}

func TestORE_StatsPubkey(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := oreClient.StatsPubkey(ctx, ore.StatsPubkeyRequest{Pubkey: fran})
	if err != nil {
		t.Fatal(err)
	}

	if res.Pubkey != fran {
		t.Errorf("pubkey=%s, want %s", res.Pubkey, fran)
	}
	if res.Rank <= 0 {
		t.Errorf("expected rank > 0, got %f", res.Rank)
	}
	if res.Followers == nil || *res.Followers <= 0 {
		t.Errorf("expected followers > 0, got %v", res.Followers)
	}
	if res.Follows == nil || *res.Follows <= 0 {
		t.Errorf("expected follows > 0, got %v", res.Follows)
	}
}

func TestORE_RankPubkeys(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := oreClient.RankPubkeys(ctx, ore.RankPubkeysRequest{
		Pubkeys: []string{calle, calle, fran, randomKey},
	})
	if err != nil {
		t.Fatal(err)
	}

	pubkeys := make([]string, len(res.Results))
	for i, r := range res.Results {
		pubkeys[i] = r.Pubkey
	}

	expected := []string{calle, fran, randomKey}
	if !slices.Equal(pubkeys, expected) {
		PrintDifference(t, pubkeys, expected)
	}
}

func TestORE_RecommendPubkeys(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := oreClient.RecommendPubkeys(ctx, ore.RecommendPubkeysRequest{Limit: 3})
	if err != nil {
		t.Fatal(err)
	}

	pubkeys := make([]string, len(res.Results))
	for i, r := range res.Results {
		pubkeys[i] = r.Pubkey
	}

	expected := []string{damus, jack_dorsey, jb55}
	if !slices.Equal(pubkeys, expected) {
		PrintDifference(t, pubkeys, expected)
	}
}

func TestORE_SearchPubkeys(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := oreClient.SearchPubkeys(ctx, ore.SearchPubkeysRequest{
		Query: "jack",
		Limit: 3,
	})
	if err != nil {
		t.Fatal(err)
	}

	pubkeys := make([]string, len(res.Results))
	for i, r := range res.Results {
		pubkeys[i] = r.Pubkey
	}

	expected := []string{jack_dorsey, jack_mallers, jack_spirko}
	if !slices.Equal(pubkeys, expected) {
		PrintDifference(t, pubkeys, expected)
	}
}
