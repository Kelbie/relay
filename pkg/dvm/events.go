// The dvm package handles parsing the request events, and responding with the appropriate events/errors.
package dvm

import (
	"context"
	"encoding/json"

	"github.com/nbd-wtf/go-nostr"
	"github.com/vertex-lab/crawler/pkg/models"
)

var (
	// kinds
	KindRelevantWhoFollow      int = 5312
	KindRecommendedFollows     int = 5313
	KindSortAuthors            int = 5314
	KindImpersonatorDetection  int = 5315
	KindDegreesOfSeparation    int = 5316
	KindVerifiedFollowersCount int = 5317
	KindVerifiedFollowers      int = 5318
	KindDVMError               int = 7000
)

// ErrorEvent() returns an unsigned nostr event for the DVM error
func ErrorEvent(errMsg, requestID, requestPubkey string) *nostr.Event {
	var tags = nostr.Tags{{"status", "error", errMsg}}
	if requestID != "" {
		tags = append(tags, nostr.Tag{"e", requestID})
	}
	if requestPubkey != "" {
		tags = append(tags, nostr.Tag{"p", requestPubkey})
	}

	return &nostr.Event{
		Content:   "",
		CreatedAt: nostr.Now(),
		Kind:      KindDVMError,
		Tags:      tags,
	}
}

// ResponseEvent() returns an unsigned nostr event used for the DVM
func ResponseEvent(res []RankResponse, requestID, requestPubkey string, requestKind int) *nostr.Event {
	var tags nostr.Tags
	if requestID != "" {
		tags = append(tags, nostr.Tag{"e", requestID})
	}
	if requestPubkey != "" {
		tags = append(tags, nostr.Tag{"p", requestPubkey})
	}

	jsonBytes, err := json.Marshal(res)
	if err != nil {
		return ErrorEvent(err.Error(), requestID, requestPubkey)
	}

	var content = string(jsonBytes)
	return &nostr.Event{
		Content:   content,
		CreatedAt: nostr.Now(),
		Kind:      requestKind + 1000,
		Tags:      tags,
	}
}

// RelevantWhoFollowEvent() returns the relevent-who-follow event from the specified args.
func RelevantWhoFollowEvent(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	args *Args) *nostr.Event {

	res, err := RelevantWhoFollow(ctx, DB, RWS, args)
	if err != nil {
		return ErrorEvent(err.Error(), args.ID, args.Pubkey)
	}

	return ResponseEvent(res, args.ID, args.Pubkey, args.Kind)
}

// RecommendedFollowsEvent() returns the recommended follows event from the specified args.
func RecommendedFollowsEvent(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	args *Args) *nostr.Event {

	res, err := RecommendedFollows(ctx, DB, RWS, args)
	if err != nil {
		return ErrorEvent(err.Error(), args.ID, args.Pubkey)
	}

	return ResponseEvent(res, args.ID, args.Pubkey, args.Kind)
}
