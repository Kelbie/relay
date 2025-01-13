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
	return &nostr.Event{
		Content:   "",
		CreatedAt: nostr.Now(),
		Kind:      KindDVMError,
		Tags: nostr.Tags{
			{"e", requestID},
			{"p", requestPubkey},
			{"status", "error", errMsg},
		},
	}
}

// ResponseEvent() returns an unsigned nostr event used for the DVM
func ResponseEvent(res []RankResponse, requestKind int, requestID, requestPubkey string) *nostr.Event {

	jsonBytes, err := json.Marshal(res)
	if err != nil {
		return ErrorEvent(err.Error(), requestID, requestPubkey)
	}

	content := string(jsonBytes)
	return &nostr.Event{
		Content:   content,
		CreatedAt: nostr.Now(),
		Kind:      requestKind + 1000,
		Tags: nostr.Tags{
			{"e", requestID},
			{"p", requestPubkey},
		},
	}
}

// RelevantWhoFollowEvent() returns the relevent-who-follow event from the specified request.
func RelevantWhoFollowEvent(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	req *nostr.Event) *nostr.Event {

	var ID string
	var pubkey string
	var kind int

	if req != nil {
		ID = req.ID
		pubkey = req.PubKey
		kind = req.Kind
	}

	args, err := Parse(req)
	if err != nil {
		return ErrorEvent(err.Error(), ID, pubkey)
	}

	res, err := RelevantWhoFollow(ctx, DB, RWS, args)
	if err != nil {
		return ErrorEvent(err.Error(), ID, pubkey)
	}

	return ResponseEvent(res, kind, ID, pubkey)
}

// RecommendedFollowsEvent() returns the recommended follows event from the specified request.
func RecommendedFollowsEvent(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	req *nostr.Event) *nostr.Event {

	var ID string
	var pubkey string
	var kind int

	if req != nil {
		ID = req.ID
		pubkey = req.PubKey
		kind = req.Kind
	}

	args, err := Parse(req)
	if err != nil {
		return ErrorEvent(err.Error(), ID, pubkey)
	}

	res, err := RecommendedFollows(ctx, DB, RWS, args)
	if err != nil {
		return ErrorEvent(err.Error(), ID, pubkey)
	}

	return ResponseEvent(res, kind, ID, pubkey)
}
