package main

import (
	"context"
	"fmt"

	"github.com/vertex-lab/relay/pkg/dvm"
	"github.com/vertex-lab/relay/pkg/eventstore"

	"github.com/nbd-wtf/go-nostr"
	"github.com/vertex-lab/crawler/pkg/models"
)

// ProcessRequest() constructs the appropriate DVM event given args and parsingErr,
// and then applies the closure function responseHandler.
func ProcessRequest(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	eventstore *eventstore.Store,
	args *dvm.Args,
	parsingErr error,
	responseHandler func(context.Context, *nostr.Event) error) error {

	if args == nil {
		return dvm.ErrNilArgs
	}

	var res *nostr.Event
	if parsingErr != nil {
		// if there are parsing errors, return them as the appropriate DVM event
		res = dvm.ErrorEvent(parsingErr.Error(), args.ID, args.Pubkey)
		return responseHandler(ctx, res)
	}

	switch args.Kind {
	case dvm.KindVerifyReputation:
		res = dvm.VerifyReputationEvent(ctx, DB, RWS, args)

	case dvm.KindRecommendFollows:
		res = dvm.RecommendFollowsEvent(ctx, DB, RWS, args)

	case dvm.KindSortAuthors:
		res = dvm.SortAuthorsEvent(ctx, DB, RWS, args)

	case dvm.KindSearchAuthors:
		res = dvm.SearchAuthorsEvent(ctx, DB, RWS, eventstore, args)

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
