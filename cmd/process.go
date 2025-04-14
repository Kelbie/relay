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
	event *nostr.Event,
	responseHandler func(context.Context, *nostr.Event) error) error {

	if event == nil {
		return fmt.Errorf("nil event pointer")
	}

	request, err := dvm.Parse(event)
	if err != nil {
		return responseHandler(ctx, dvm.ErrorEvent(err, dvm.NewRecord(event)))
	}

	paid, err := limiter.Pay(request.Pubkey, cost(request))
	if err != nil {
		return responseHandler(ctx, dvm.ErrorEvent(err, request.Record))
	}

	if !paid {
		return responseHandler(ctx, dvm.ErrorEvent(dvm.ErrNoCredits, request.Record))
	}

	return ProcessRequest(ctx, DB, RWS, eventStore, request, responseHandler)
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
	request, err := req.Parse(filter)
	if err != nil {
		return responseHandler(ctx, dvm.ErrorEvent(err, record))
	}

	return ProcessRequest(ctx, DB, RWS, eventStore, request, responseHandler)
}

func ProcessRequest(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	eventStore *eventstore.Store,
	request *dvm.Request,
	responseHandler func(context.Context, *nostr.Event) error) error {

	var response dvm.Response
	var err error

	switch request.Kind {
	case dvm.KindVerifyReputation:
		response, err = dvm.VerifyReputation(ctx, DB, RWS, request)

	case dvm.KindRecommendFollows:
		response, err = dvm.RecommendFollows(ctx, DB, RWS, request)

	case dvm.KindSortProfiles:
		response, err = dvm.SortProfiles(ctx, DB, RWS, request)

	case dvm.KindSearchProfiles:
		response, err = dvm.SearchProfiles(ctx, DB, RWS, eventStore, request)

	default:
		err = fmt.Errorf("%w: %v", dvm.ErrInvalidKind, request.Kind)
	}

	if err != nil {
		return responseHandler(ctx, dvm.ErrorEvent(err, request.Record))
	}

	requestCounter.Add(1)
	if requestCounter.Load()%500 == 0 {
		log.Info("processed %d requests", requestCounter.Load())
	}

	return responseHandler(ctx, dvm.ResponseEvent(response, request))
}

// This function estimates the cost of processing a request with the provided params.
func cost(r *dvm.Request) int {
	switch r.Sort {
	case dvm.Personalized:
		return 10

	default:
		return 1
	}
}
