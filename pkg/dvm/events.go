// The dvm package handles parsing the request events, and responding with the appropriate events/errors.
package dvm

import (
	"context"
	"encoding/json"
	"relay/pkg/response"

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

// ErrorEvent() returns an unsigned nostr event for the DVM error response.
func ErrorEvent(err error, request *nostr.Event) *nostr.Event {
	var ID string
	var pubkey string
	var errMsg string

	if request != nil {
		ID = request.ID
		pubkey = request.PubKey
	}

	if err != nil {
		errMsg = err.Error()
	}

	return &nostr.Event{
		Content:   "",
		CreatedAt: nostr.Now(),
		Kind:      KindDVMError,
		Tags: nostr.Tags{
			{"e", ID},
			{"p", pubkey},
			{"status", "error", errMsg},
		},
	}
}

// ResponseEvent() returns an unsigned nostr event used for the DVM response.
func ResponseEvent(res []response.T, request *nostr.Event) *nostr.Event {
	var ID string
	var pubkey string
	var kind int

	if request != nil {
		ID = request.ID
		pubkey = request.PubKey
		kind = request.Kind
	}

	jsonBytes, err := json.Marshal(res)
	if err != nil {
		return ErrorEvent(err, request)
	}

	content := string(jsonBytes)
	return &nostr.Event{
		Content:   content,
		CreatedAt: nostr.Now(),
		Kind:      kind + 1000,
		Tags: nostr.Tags{
			{"e", ID},
			{"p", pubkey},
		},
	}
}

// RelevantWhoFollowEvent() returns the relevent-who-follow event from the specified request.
func RelevantWhoFollowEvent(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	req *nostr.Event) *nostr.Event {

	args, err := ParseArgs(req)
	if err != nil {
		return ErrorEvent(err, req)
	}

	res, err := response.RelevantWhoFollow(ctx, DB, RWS, args)
	if err != nil {
		return ErrorEvent(err, req)
	}

	return ResponseEvent(res, req)
}

// RecommendedFollowsEvent() returns the recommended follows event from the specified request.
func RecommendedFollowsEvent(
	ctx context.Context,
	DB models.Database,
	RWS models.RandomWalkStore,
	req *nostr.Event) *nostr.Event {

	args, err := ParseArgs(req)
	if err != nil {
		return ErrorEvent(err, req)
	}

	res, err := response.RecommendedFollows(ctx, DB, RWS, args)
	if err != nil {
		return ErrorEvent(err, req)
	}

	return ResponseEvent(res, req)
}
