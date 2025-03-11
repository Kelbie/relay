package main

import (
	"context"
	"fmt"

	"github.com/vertex-lab/relay/pkg/dvm"
	"github.com/vertex-lab/relay/pkg/eventstore"
	"github.com/vertex-lab/relay/pkg/req"

	"github.com/nbd-wtf/go-nostr"
	"github.com/vertex-lab/crawler/pkg/models"
)

func HandleDVMRequest(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	eventStore *eventstore.Store,
	request *nostr.Event,
	responseHandler func(context.Context, *nostr.Event) error) error {

	if request == nil {
		return fmt.Errorf("nil event pointer")
	}

	record := dvm.Record{ID: request.ID, Pubkey: request.PubKey, Kind: request.Kind}
	params, err := dvm.Parse(request)
	if err != nil {
		return responseHandler(ctx, dvm.ErrorEvent(err, record))
	}

	paid, err := limiter.Pay(request.PubKey, cost(params))
	if err != nil {
		return responseHandler(ctx, dvm.ErrorEvent(err, record))
	}

	if !paid {
		return responseHandler(ctx, dvm.ErrorEvent(dvm.ErrNoCredits, record))
	}

	return ProcessRequest(ctx, DB, RWS, eventStore, params, record, responseHandler)
}

func HandleREQRequest(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	eventStore *eventstore.Store,
	filter *nostr.Filter,
	responseHandler func(context.Context, *nostr.Event) error) error {

	if filter == nil {
		return req.ErrNilFilter
	}

	if err := req.ValidateFilter(filter); err != nil {
		// if the filter doesn't match the valid format "kinds:<dvm_response_kind>, 7000",
		// return the error as a NOTICE and not as a kind:7000 to make sure the customer receives it.
		return err
	}

	record := dvm.Record{Kind: filter.Kinds[0] - 1000}
	params, err := req.Parse(filter)
	if err != nil {
		return responseHandler(ctx, dvm.ErrorEvent(err, record))
	}

	return ProcessRequest(ctx, DB, RWS, eventStore, params, record, responseHandler)
}

func ProcessRequest(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	eventStore *eventstore.Store,
	params dvm.Params,
	record dvm.Record,
	responseHandler func(context.Context, *nostr.Event) error) error {

	var res dvm.PubkeyRanks
	var err error

	switch record.Kind {
	case dvm.KindVerifyReputation:
		args, err := params.ToVerifyReputationArgs()
		if err != nil {
			return err
		}
		res, err = dvm.VerifyReputation(ctx, DB, RWS, args)

	case dvm.KindRecommendFollows:
		args, err := params.ToRecommendFollowsArgs()
		if err != nil {
			return err
		}
		res, err = dvm.RecommendFollows(ctx, DB, RWS, args)

	case dvm.KindSortProfiles:
		args, err := params.ToSortProfilesArgs()
		if err != nil {
			return err
		}
		res, err = dvm.SortProfiles(ctx, DB, RWS, args)

	case dvm.KindSearchProfiles:
		args, err := params.ToSearchProfilesArgs()
		if err != nil {
			return err
		}
		res, err = dvm.SearchProfiles(ctx, DB, RWS, eventStore, args)

	default:
		err = fmt.Errorf("%w: %v", dvm.ErrInvalidKind, record.Kind)
	}

	if err != nil {
		return responseHandler(ctx, dvm.ErrorEvent(err, record))
	}

	requestCounter.Add(1)
	if requestCounter.Load()%500 == 0 {
		log.Info("processed %d requests", requestCounter.Load())
	}

	return responseHandler(ctx, dvm.ResponseEvent(res, record))
}

// This function estimates the cost of processing a request with the provided params.
func cost(params dvm.Params) int {
	switch params.Sort {
	case dvm.Personalized:
		return 100

	default:
		return 10
	}
}
