// The dvm package handles parsing the DVM requests, and responding with the appropriate DVM response / DVM error.
package dvm

import (
	"context"
	"encoding/json"

	"github.com/nbd-wtf/go-nostr"
	"github.com/vertex-lab/crawler/pkg/models"
	"github.com/vertex-lab/relay/pkg/eventstore"
)

var (
	// kinds
	KindVerifyReputation int = 5312
	KindRecommendFollows int = 5313
	KindSortAuthors      int = 5314
	KindSearchAuthors    int = 5315
	KindDVMError         int = 7000
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

	if len(res) >= 1 && requestKind == KindVerifyReputation && requestID == "" {
		// this is a nasty trick to mantain backwards compatibility with Zapstore,
		// that should be removed as soon as Zapstore upgreades to the new format for VerifyReputation.
		// requestID == "" iff REQ is used.
		res = res[1:]
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

// VerifyReputationEvent() returns the verify reputation event from the specified args.
func VerifyReputationEvent(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	args *Args) *nostr.Event {

	res, err := VerifyReputation(ctx, DB, RWS, args)
	if err != nil {
		return ErrorEvent(err.Error(), args.ID, args.Pubkey)
	}

	return ResponseEvent(res, args.ID, args.Pubkey, args.Kind)
}

// SortAuthorsEvent() returns the sorted authors event from the specified args.
func SortAuthorsEvent(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	args *Args) *nostr.Event {

	res, err := SortAuthors(ctx, DB, RWS, args)
	if err != nil {
		return ErrorEvent(err.Error(), args.ID, args.Pubkey)
	}

	return ResponseEvent(res, args.ID, args.Pubkey, args.Kind)
}

// RecommendFollowsEvent() returns the recommended follows event from the specified args.
func RecommendFollowsEvent(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	args *Args) *nostr.Event {

	res, err := RecommendFollows(ctx, DB, RWS, args)
	if err != nil {
		return ErrorEvent(err.Error(), args.ID, args.Pubkey)
	}

	return ResponseEvent(res, args.ID, args.Pubkey, args.Kind)
}

// SearchAuthorsEvent() returns the sorted authors event from the specified args.
func SearchAuthorsEvent(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	eventStore *eventstore.Store,
	args *Args) *nostr.Event {

	res, err := SearchAuthors(ctx, DB, RWS, eventStore, args)
	if err != nil {
		return ErrorEvent(err.Error(), args.ID, args.Pubkey)
	}

	return ResponseEvent(res, args.ID, args.Pubkey, args.Kind)
}
