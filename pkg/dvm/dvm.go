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

// ErrorEvent() returns an unsigned nostr error event from the specified error and request.
func ErrorEvent(err error, request *nostr.Event) *nostr.Event {
	event := nostr.Event{
		Content:   "",
		CreatedAt: nostr.Now(),
		Kind:      KindDVMError,
		Tags: nostr.Tags{
			nostr.Tag{"e", request.ID},
			nostr.Tag{"p", request.PubKey},
			{"status", "error", err.Error()},
		},
	}

	return &event
}

// ResponseEvent() returns an unsigned nostr event used for a DVM response.
func ResponseEvent(result any, request *nostr.Event) *nostr.Event {
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return ErrorEvent(err, request)
	}

	content := string(jsonBytes)
	event := nostr.Event{
		Content:   content,
		CreatedAt: nostr.Now(),
		Kind:      request.Kind + 1000,
		Tags: nostr.Tags{
			nostr.Tag{"e", request.ID},
			nostr.Tag{"p", request.PubKey},
		},
	}

	return &event
}
