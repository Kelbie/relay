package req

import (
	"context"
	"relay/pkg/dvm"

	"github.com/nbd-wtf/go-nostr"
	"github.com/vertex-lab/crawler/pkg/models"
)

// RelevantWhoFollowEvent() returns the relevent-who-follow event from the specified request.
func RelevantWhoFollowEvent(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	filter *nostr.Filter) *nostr.Event {

	var kind int
	if filter != nil {
		kind = filter.Kinds[0]
	}

	args, err := Parse(filter)
	if err != nil {
		return dvm.ErrorEvent(err.Error(), "", "")
	}

	res, err := dvm.RelevantWhoFollow(ctx, DB, RWS, args)
	if err != nil {
		return dvm.ErrorEvent(err.Error(), "", "")
	}

	return dvm.ResponseEvent(res, kind, "", "")
}

// RecommendedFollowsEvent() returns the recommended follows event from the specified request.
func RecommendedFollowsEvent(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	filter *nostr.Filter) *nostr.Event {

	var kind int
	if filter != nil {
		kind = filter.Kinds[0]
	}

	args, err := Parse(filter)
	if err != nil {
		return dvm.ErrorEvent(err.Error(), "", "")
	}

	res, err := dvm.RecommendedFollows(ctx, DB, RWS, args)
	if err != nil {
		return dvm.ErrorEvent(err.Error(), "", "")
	}

	return dvm.ResponseEvent(res, kind, "", "")
}
