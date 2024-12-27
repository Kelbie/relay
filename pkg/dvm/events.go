// The dvm package handles parsing the request events, and responding with the appropriate events/errors.
package dvm

import (
	"encoding/json"

	"github.com/nbd-wtf/go-nostr"
)

var (
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

	event := nostr.Event{
		Content:   "",
		CreatedAt: nostr.Now(),
		Kind:      KindDVMError,
		Tags: nostr.Tags{
			{"e", ID},
			{"p", pubkey},
			{"status", "error", errMsg},
		},
	}

	return &event
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
	event := nostr.Event{
		Content:   content,
		CreatedAt: nostr.Now(),
		Kind:      kind + 1000,
		Tags: nostr.Tags{
			{"e", ID},
			{"p", pubkey},
		},
	}

	return &event
}
