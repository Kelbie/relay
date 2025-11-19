package dvm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/nbd-wtf/go-nostr"
	"github.com/vertex-lab/relay/pkg/core"
)

type Handler struct {
	Service   *core.Service
	SecretKey string
}

func (h Handler) Process(ctx context.Context, request *nostr.Event) *nostr.Event {
	response := h.process(ctx, request)
	err := response.Sign(h.SecretKey)
	if err != nil {
		// the handler failed to sign the response, likely caused by an invalid secret key.
		// This is an unrecoverable error since all responses must be signed.
		panic(fmt.Errorf("dvm.Handler: failed to sign: %w", err))
	}
	return response
}

func (h Handler) process(ctx context.Context, request *nostr.Event) *nostr.Event {
	args, err := Parse(request)
	if err != nil {
		return Error(request, err)
	}

	if err = args.Normalize(); err != nil {
		return Error(request, err)
	}

	switch args := args.(type) {
	case *core.VerifyReputationArgs:
		result, err := h.Service.VerifyReputation(ctx, *args)
		if err != nil {
			return Error(request, err)
		}

		response, err := VerifyReputation(request, *args, result)
		if err != nil {
			return Error(request, err)
		}
		return response

	case *core.RecommendFollowsArgs:
		result, err := h.Service.RecommendFollows(ctx, *args)
		if err != nil {
			return Error(request, err)
		}

		response, err := RecommendFollows(request, *args, result)
		if err != nil {
			return Error(request, err)
		}
		return response

	case *core.RankProfilesArgs:
		result, err := h.Service.RankProfiles(ctx, *args)
		if err != nil {
			return Error(request, err)
		}

		response, err := RankProfiles(request, *args, result)
		if err != nil {
			return Error(request, err)
		}
		return response

	case *core.SearchProfilesArgs:
		result, err := h.Service.SearchProfiles(ctx, *args)
		if err != nil {
			return Error(request, err)
		}

		response, err := SearchProfiles(request, *args, result)
		if err != nil {
			return Error(request, err)
		}
		return response

	default:
		slog.Error("dvm.Handler received an unknown type")
		return Error(request, fmt.Errorf("%w: %w", core.ErrInternal, core.ErrUnsupportedArgs))
	}
}

func VerifyReputation(
	request *nostr.Event,
	args core.VerifyReputationArgs,
	result core.VerifyReputationResult) (*nostr.Event, error) {

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
	args core.RecommendFollowsArgs,
	result core.RecommendFollowsResult) (*nostr.Event, error) {

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
	args core.RankProfilesArgs,
	result core.RankProfilesResult) (*nostr.Event, error) {

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
	args core.SearchProfilesArgs,
	result core.SearchProfilesResult) (*nostr.Event, error) {

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
