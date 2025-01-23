package main

import (
	"context"
	"fmt"
	"relay/pkg/dvm"

	"github.com/nbd-wtf/go-nostr"
	"github.com/vertex-lab/crawler/pkg/models"
)

// ProcessRequest() constructs the appropriate DVM event given args and parsingErr,
// and then applies the closure function responseHandler.
func ProcessRequest(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	args *dvm.Args,
	parsingErr error,
	responseHandler func(context.Context, *nostr.Event) error) error {

	if args == nil {
		log.Error("error processing request after parsing: %v", dvm.ErrNilArgs)
		return dvm.ErrNilArgs
	}

	var res *nostr.Event
	if parsingErr != nil {
		// if there are parsing errors, return them as the appropriate DVM event
		res = dvm.ErrorEvent(parsingErr.Error(), args.ID, args.Pubkey)
		return responseHandler(ctx, res)
	}

	switch args.Kind {
	case dvm.KindRelevantWhoFollow:
		res = dvm.RelevantWhoFollowEvent(ctx, DB, RWS, args)

	case dvm.KindRecommendedFollows:
		res = dvm.RecommendedFollowsEvent(ctx, DB, RWS, args)

	case dvm.KindSortAuthors, dvm.KindImpersonatorDetection, dvm.KindDegreesOfSeparation, dvm.KindVerifiedFollowersCount, dvm.KindVerifiedFollowers:
		res = &nostr.Event{Content: "This service is coming soon", Kind: args.Kind + 1000}

	default:
		return fmt.Errorf("%w: %v", dvm.ErrInvalidKind, args.Kind)
	}

	// log how many requests have been processed so far
	requestCounter.Add(1)
	if requestCounter.Load()%250 == 0 {
		log.Info("processed %v requests", requestCounter.Load())
	}

	return responseHandler(ctx, res)
}
