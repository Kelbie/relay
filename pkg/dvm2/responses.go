package dvm2

import (
	"encoding/json"
	"strconv"

	"github.com/nbd-wtf/go-nostr"
	"github.com/vertex-lab/relay/pkg/service"
)

func VerifyReputation(
	request *nostr.Event,
	args service.VerifyReputationArgs,
	result service.VerifyReputationResult) (*nostr.Event, error) {

	array := make([]any, 1+len(result.TopFollowers))
	array[0] = result.Target
	for i := range result.TopFollowers {
		array[i+1] = result.TopFollowers[i]
	}

	content, err := json.Marshal(array)
	if err != nil {
		return nil, err
	}

	return &nostr.Event{
		Content:   string(content),
		CreatedAt: nostr.Now(),
		Kind:      KindVerifyReputation + 1000,
		Tags: nostr.Tags{
			{"e", request.ID},
			{"p", request.PubKey},
			{"nodes", strconv.Itoa(result.Nodes)},
			{"sort", args.Sort},
			{"source", args.Source},
		},
	}, nil
}

func RecommendFollows(
	request *nostr.Event,
	args service.RecommendFollowsArgs,
	result service.RecommendFollowsResult) (*nostr.Event, error) {

	content, err := json.Marshal(result.Recommendations)
	if err != nil {
		return nil, err
	}

	return &nostr.Event{
		Content:   string(content),
		CreatedAt: nostr.Now(),
		Kind:      KindRecommendFollows + 1000,
		Tags: nostr.Tags{
			{"e", request.ID},
			{"p", request.PubKey},
			{"nodes", strconv.Itoa(result.Nodes)},
			{"sort", args.Sort},
			{"source", args.Source},
		},
	}, nil
}

func RankProfiles(
	request *nostr.Event,
	args service.RankProfilesArgs,
	result service.RankProfilesResult) (*nostr.Event, error) {

	content, err := json.Marshal(result.Profiles)
	if err != nil {
		return nil, err
	}

	return &nostr.Event{
		Content:   string(content),
		CreatedAt: nostr.Now(),
		Kind:      KindRankProfiles + 1000,
		Tags: nostr.Tags{
			{"e", request.ID},
			{"p", request.PubKey},
			{"nodes", strconv.Itoa(result.Nodes)},
			{"sort", args.Sort},
			{"source", args.Source},
		},
	}, nil
}

func SearchProfiles(
	request *nostr.Event,
	args service.SearchProfilesArgs,
	result service.SearchProfilesResult) (*nostr.Event, error) {

	content, err := json.Marshal(result.Results)
	if err != nil {
		return nil, err
	}

	return &nostr.Event{
		Content:   string(content),
		CreatedAt: nostr.Now(),
		Kind:      KindSearchProfiles + 1000,
		Tags: nostr.Tags{
			{"e", request.ID},
			{"p", request.PubKey},
			{"nodes", strconv.Itoa(result.Nodes)},
			{"sort", args.Sort},
			{"source", args.Source},
		},
	}, nil
}

// Error returns an unsigned nostr event for the DVM error.
func Error(request *nostr.Event, err error) *nostr.Event {
	return &nostr.Event{
		CreatedAt: nostr.Now(),
		Kind:      KindDVMError,
		Tags: nostr.Tags{
			{"e", request.ID},
			{"p", request.PubKey},
			{"status", "error", err.Error()},
		},
	}
}
