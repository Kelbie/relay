package dvm2

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/nbd-wtf/go-nostr"
	"github.com/vertex-lab/relay/pkg/rate"
	"github.com/vertex-lab/relay/pkg/service"
)

type Handler struct {
	Service   *service.Service
	Limiter   rate.Limiter
	SecretKey string
}

func (h Handler) Process(ctx context.Context, request *nostr.Event) *nostr.Event {
	response := h.process(ctx, request)
	err := response.Sign(h.SecretKey)
	if err != nil {
		// this is an unrecoverable error, likely caused by the handler having
		// an invalid secret key. No responses can be made because they all must be signed.
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

	if !h.Limiter.Allow(request.PubKey, args.Cost()) {
		return Error(request, service.ErrNoCredits)
	}

	switch args := args.(type) {
	case *service.VerifyReputationArgs:
		result, err := h.Service.VerifyReputation(ctx, *args)
		if err != nil {
			return Error(request, err)
		}

		response, err := VerifyReputation(request, *args, result)
		if err != nil {
			return Error(request, err)
		}
		return response

	case *service.RecommendFollowsArgs:
		result, err := h.Service.RecommendFollows(ctx, *args)
		if err != nil {
			return Error(request, err)
		}

		response, err := RecommendFollows(request, *args, result)
		if err != nil {
			return Error(request, err)
		}
		return response

	case *service.RankProfilesArgs:
		result, err := h.Service.RankProfiles(ctx, *args)
		if err != nil {
			return Error(request, err)
		}

		response, err := RankProfiles(request, *args, result)
		if err != nil {
			return Error(request, err)
		}
		return response

	case *service.SearchProfilesArgs:
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
		return Error(request, fmt.Errorf("%w: %w", service.ErrInternal, service.ErrUnsupportedArgs))
	}
}

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
