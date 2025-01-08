package main

import (
	"context"
	"relay/pkg/dvm"

	"github.com/fiatjaf/khatru"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip46"
	"github.com/redis/go-redis/v9"
	"github.com/vertex-lab/crawler/pkg/database/redisdb"
	"github.com/vertex-lab/crawler/pkg/store/redistore"
	"github.com/vertex-lab/crawler/pkg/utils/logger"
)

// ProcessRequests() consumes from the request channel, produces a response event,
// signs it with the bunker and broadcasts it.
func ProcessRequests(
	ctx context.Context,
	logger *logger.Aggregate,
	redis *redis.Client,
	bunker *nip46.BunkerClient,
	relay *khatru.Relay,
	reqChan <-chan *nostr.Event,
) {

	DB, err := redisdb.NewDatabaseConnection(ctx, redis)
	if err != nil {
		logger.Error("failed to connect to the redis database")
		panic(err)
	}

	RWS, err := redistore.NewRWSConnection(ctx, redis)
	if err != nil {
		logger.Error("failed to connect to the redis store")
		panic(err)
	}

	for {
		select {
		case <-ctx.Done():
			logger.Warn("Stopped processing the event.")
			return

		case req, ok := <-reqChan:
			if !ok {
				logger.Warn("Request channel closed, stopped processing.")
				return
			}

			var res *nostr.Event
			switch req.Kind {
			case dvm.KindRelevantWhoFollow:
				res = dvm.RelevantWhoFollowEvent(ctx, DB, RWS, req)

			case dvm.KindRecommendedFollows:
				res = dvm.RecommendedFollowsEvent(ctx, DB, RWS, req)

			case dvm.KindSortAuthors:
				res = &nostr.Event{Content: "this dvm is WIP"}
			case dvm.KindImpersonatorDetection:
				res = &nostr.Event{Content: "this dvm is WIP"}
			case dvm.KindDegreesOfSeparation:
				res = &nostr.Event{Content: "this dvm is WIP"}
			case dvm.KindVerifiedFollowersCount:
				res = &nostr.Event{Content: "this dvm is WIP"}
			case dvm.KindVerifiedFollowers:
				res = &nostr.Event{Content: "this dvm is WIP"}
			default:
				logger.Error("unwanted kind: %v", req.Kind)
			}

			bunker.SignEvent(ctx, res)
			relay.BroadcastEvent(res)
		}
	}
}
