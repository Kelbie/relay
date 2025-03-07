// The dvm package handles parsing the DVM requests, and responding with the appropriate DVM response / DVM error.
package dvm

import (
	"encoding/json"

	"github.com/nbd-wtf/go-nostr"
)

var (
	// kinds
	KindVerifyReputation int = 5312
	KindRecommendFollows int = 5313
	KindSortProfiles     int = 5314
	KindSearchProfiles   int = 5315
	KindDVMError         int = 7000
)

// Record encapsulates the relevant fields for identifying the request.
type Record struct {
	ID     string
	Pubkey string
	Kind   int
}

func (r Record) ToTags() nostr.Tags {
	var tags nostr.Tags
	if r.ID != "" {
		tags = append(tags, nostr.Tag{"e", r.ID})
	}

	if r.Pubkey != "" {
		tags = append(tags, nostr.Tag{"p", r.Pubkey})
	}

	return tags
}

// ErrorEvent() returns an unsigned nostr event for the DVM error
func ErrorEvent(err error, rec Record) *nostr.Event {
	return &nostr.Event{
		Content:   "",
		CreatedAt: nostr.Now(),
		Kind:      KindDVMError,
		Tags:      append(rec.ToTags(), nostr.Tag{"status", "error", err.Error()}),
	}
}

// ResponseEvent() returns an unsigned nostr event used for the DVM
func ResponseEvent(response PubkeyRanks, rec Record) *nostr.Event {
	if len(response) >= 1 && rec.Kind == KindVerifyReputation && rec.ID == "" {
		// this is a nasty trick to mantain backwards compatibility with Zapstore,
		// that should be removed as soon as Zapstore upgrades to the new format for VerifyReputation.
		// rec.ID == "" iff REQ is used.
		response = response[1:]
	}

	json, err := json.Marshal(response)
	if err != nil {
		return ErrorEvent(err, rec)
	}

	return &nostr.Event{
		Content:   string(json),
		CreatedAt: nostr.Now(),
		Kind:      rec.Kind + 1000,
		Tags:      rec.ToTags(),
	}
}
