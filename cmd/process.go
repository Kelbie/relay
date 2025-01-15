package main

import (
	"context"
	"relay/pkg/dvm"
	"relay/pkg/req"

	"github.com/nbd-wtf/go-nostr"
	"github.com/redis/go-redis/v9"
	"github.com/vertex-lab/crawler/pkg/database/redisdb"
	"github.com/vertex-lab/crawler/pkg/store/redistore"
	"github.com/vertex-lab/crawler/pkg/utils/logger"
)

// TODO; handle the requests context in both of the Processes.

// ProcessRequests() consumes from the queues, produces a response event,
// and handle the response in the specified way.
func ProcessRequests(
	ctx context.Context,
	logger *logger.Aggregate,
	redis *redis.Client,
	DVMQueue <-chan *nostr.Event,
	filterQueue <-chan *nostr.Filter,
	responseHandler func(ctx context.Context, res *nostr.Event) error,
) {

	// initialize connections to redis
	DB, err := redisdb.NewDatabaseConnection(ctx, redis)
	if err != nil {
		panic(err)
	}

	RWS, err := redistore.NewRWSConnection(ctx, redis)
	if err != nil {
		panic(err)
	}

	logger.Info("started processing requests...")

	for {
		select {
		case <-ctx.Done():
			logger.Warn("Stopped processing requests.")
			return

		case event, ok := <-DVMQueue:
			if !ok {
				logger.Warn("DVM queue closed, stopped processing.")
				return
			}

			var res *nostr.Event
			switch event.Kind {
			case dvm.KindRelevantWhoFollow:
				res = dvm.RelevantWhoFollowEvent(ctx, DB, RWS, event)

			case dvm.KindRecommendedFollows:
				res = dvm.RecommendedFollowsEvent(ctx, DB, RWS, event)

			case dvm.KindSortAuthors, dvm.KindImpersonatorDetection, dvm.KindDegreesOfSeparation, dvm.KindVerifiedFollowersCount, dvm.KindVerifiedFollowers:
				res = &nostr.Event{Content: "This service is coming soon", Kind: event.Kind + 1000}

			default:
				logger.Error("unwanted kind: %v", event.Kind)
				continue
			}

			if err := responseHandler(ctx, res); err != nil {
				logger.Error("DVM response failed: %v", err)
			}

		case filter, ok := <-filterQueue:
			if !ok {
				logger.Warn("filter queue closed, stopped processing.")
				return
			}

			// the kinds must match this format: <dvm_kind>, 7000 (dvm error)
			if len(filter.Kinds) != 2 || filter.Kinds[1] != dvm.KindDVMError {
				logger.Warn("invalid filter: %v", filter)
				continue
			}

			var kind int = filter.Kinds[0]
			var res *nostr.Event
			switch kind {
			case dvm.KindRelevantWhoFollow + 1000:
				res = req.RelevantWhoFollowEvent(ctx, DB, RWS, filter)

			case dvm.KindRecommendedFollows + 1000:
				res = req.RecommendedFollowsEvent(ctx, DB, RWS, filter)

			case dvm.KindSortAuthors + 1000, dvm.KindImpersonatorDetection + 1000, dvm.KindDegreesOfSeparation + 1000, dvm.KindVerifiedFollowersCount + 1000, dvm.KindVerifiedFollowers + 1000:
				res = &nostr.Event{Content: "This service is coming soon", Kind: kind}

			default:
				logger.Error("unwanted kind: %v", kind)
				continue
			}

			if err := responseHandler(ctx, res); err != nil {
				logger.Error("REQ response failed: %v", err)
			}
		}
	}
}
