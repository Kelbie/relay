package dvm

import (
	"reflect"
	"testing"

	"github.com/nbd-wtf/go-nostr"
	"github.com/vertex-lab/relay/pkg/core"
)

func TestVerifyReputation(t *testing.T) {
	request := &nostr.Event{ID: "aaa", PubKey: "bbb"}
	args := core.VerifyReputationArgs{}
	args.Sort = "sort"
	args.Source = "source"

	result := core.VerifyReputationResult{}
	result.Nodes = 10
	result.Target.Pubkey = "target"
	result.Target.Rank = 0.1
	result.Target.Follows = 69
	result.Target.Followers = 420

	follower := core.Profile{Pubkey: "follower", Rank: 0.3}
	result.TopFollowers = append(result.TopFollowers, follower)

	expected := &nostr.Event{
		Kind:      6312,
		CreatedAt: nostr.Now(),
		Content:   "[{\"pubkey\":\"target\",\"rank\":0.1,\"follows\":69,\"followers\":420},{\"pubkey\":\"follower\",\"rank\":0.3}]",
		Tags:      nostr.Tags{{"e", "aaa"}, {"p", "bbb"}, {"nodes", "10"}, {"sort", "sort"}, {"source", "source"}},
	}

	response, err := VerifyReputation(request, args, result)
	if err != nil {
		t.Fatalf("expected error nil, got %v", err)
	}

	if !reflect.DeepEqual(response, expected) {
		t.Fatalf("expected response, %v, got %v", expected, response)
	}
}

func TestRecommendFollows(t *testing.T) {
	request := &nostr.Event{ID: "aaa", PubKey: "bbb"}
	args := core.RecommendFollowsArgs{}
	args.Sort = "sort"
	args.Source = "source"

	result := core.RecommendFollowsResult{}
	result.Nodes = 10
	recommended := core.Profile{Pubkey: "bro", Rank: 0.3}
	result.Recommendations = append(result.Recommendations, recommended)

	expected := &nostr.Event{
		Kind:      6313,
		CreatedAt: nostr.Now(),
		Content:   "[{\"pubkey\":\"bro\",\"rank\":0.3}]",
		Tags:      nostr.Tags{{"e", "aaa"}, {"p", "bbb"}, {"nodes", "10"}, {"sort", "sort"}, {"source", "source"}},
	}

	response, err := RecommendFollows(request, args, result)
	if err != nil {
		t.Fatalf("expected error nil, got %v", err)
	}

	if !reflect.DeepEqual(response, expected) {
		t.Fatalf("expected response, %v, got %v", expected, response)
	}
}

func TestRankProfiles(t *testing.T) {
	request := &nostr.Event{ID: "aaa", PubKey: "bbb"}
	args := core.RankProfilesArgs{}
	args.Sort = "sort"
	args.Source = "source"

	result := core.RankProfilesResult{}
	result.Nodes = 10
	result.Profiles = []core.Profile{
		{Pubkey: "first", Rank: 0.3},
		{Pubkey: "second", Rank: 0.2},
		{Pubkey: "third", Rank: 0.1},
	}

	expected := &nostr.Event{
		Kind:      6314,
		CreatedAt: nostr.Now(),
		Content:   "[{\"pubkey\":\"first\",\"rank\":0.3},{\"pubkey\":\"second\",\"rank\":0.2},{\"pubkey\":\"third\",\"rank\":0.1}]",
		Tags:      nostr.Tags{{"e", "aaa"}, {"p", "bbb"}, {"nodes", "10"}, {"sort", "sort"}, {"source", "source"}},
	}

	response, err := RankProfiles(request, args, result)
	if err != nil {
		t.Fatalf("expected error nil, got %v", err)
	}

	if !reflect.DeepEqual(response, expected) {
		t.Fatalf("expected response, %v, got %v", expected, response)
	}
}

func TestSearchProfiles(t *testing.T) {
	request := &nostr.Event{ID: "aaa", PubKey: "bbb"}
	args := core.SearchProfilesArgs{}
	args.Sort = "sort"
	args.Source = "source"

	result := core.SearchProfilesResult{}
	result.Nodes = 10
	result.Results = []core.Profile{
		{Pubkey: "first", Rank: 0.3},
		{Pubkey: "second", Rank: 0.2},
		{Pubkey: "third", Rank: 0.1},
	}

	expected := &nostr.Event{
		Kind:      6315,
		CreatedAt: nostr.Now(),
		Content:   "[{\"pubkey\":\"first\",\"rank\":0.3},{\"pubkey\":\"second\",\"rank\":0.2},{\"pubkey\":\"third\",\"rank\":0.1}]",
		Tags:      nostr.Tags{{"e", "aaa"}, {"p", "bbb"}, {"nodes", "10"}, {"sort", "sort"}, {"source", "source"}},
	}

	response, err := SearchProfiles(request, args, result)
	if err != nil {
		t.Fatalf("expected error nil, got %v", err)
	}

	if !reflect.DeepEqual(response, expected) {
		t.Fatalf("expected response, %v, got %v", expected, response)
	}
}
