// The dvm package handles parsing the request events, and responding with the appropriate events/errors.
package dvm

import (
	"context"
	"encoding/json"
	"errors"

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

var (
	// parsing errors
	ErrNilRequest        error = errors.New("nil request pointer")
	ErrUnknownParameter  error = errors.New("parameter must be one between 'source', 'target', 'sort', 'distance', 'limit'")
	ErrBadlyFormattedTag error = errors.New("tag should be 'param, <key>, <val>'")
	ErrBadlyFormattedKey error = errors.New("badly formatted key")
	ErrBadlyFormattedInt error = errors.New("badly formatted unsigned integer")

	// value errors
	ErrInvalidSortOption error = errors.New("sort must be one between 'global', 'personalized'")
	ErrInvalidTargets    error = errors.New("invalid targets")
	ErrInvalidLimit      error = errors.New("invalid limit")
	ErrInvalidDistance   error = errors.New("invalid distance")

	// internal system errors
	ErrComputationFailed error = errors.New("DVM computation failed")
	ErrNilArgs           error = errors.New("nil args pointer")
	ErrKeyNotFound       error = errors.New("pubkey was not found")
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
func ResponseEvent(result []RankResponse, request *nostr.Event) *nostr.Event {
	var ID string
	var pubkey string
	var kind int

	if request != nil {
		ID = request.ID
		pubkey = request.PubKey
		kind = request.Kind
	}

	jsonBytes, err := json.Marshal(result)
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

	res, err := RelevantWhoFollow(ctx, DB, RWS, args)
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

	res, err := RecommendedFollows(ctx, DB, RWS, args)
	if err != nil {
		return ErrorEvent(err, req)
	}

	return ResponseEvent(res, req)
}
