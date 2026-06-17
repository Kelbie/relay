package nip85

import (
	"math"
	"slices"
	"strconv"

	"github.com/nbd-wtf/go-nostr"
	"github.com/vertex-lab/relay/pkg/core"
)

const Kind = 30382

// Event generates an unsigned NIP-85 kind:30382 assertion events for the given profile.
func Event(pubkey string, rank int) nostr.Event {
	return nostr.Event{
		Kind:      Kind,
		CreatedAt: nostr.Now(),
		Tags: nostr.Tags{
			{"d", pubkey},
			{"rank", strconv.Itoa(rank)},
		},
	}
}

// IsQuery returns true when the filter is a NIP-85 trusted-assertion request:
// kinds=[30382], authors contains the relay's own pubkey, and #d lists target pubkeys.
func IsQuery(filters nostr.Filters, providerPubkey string) bool {
	if len(filters) != 1 {
		return false
	}
	f := filters[0]
	return slices.Equal(f.Kinds, []int{Kind}) &&
		slices.Contains(f.Authors, providerPubkey) &&
		len(f.Tags["d"]) > 0
}

// Rank returns a more "natural" score from 0-100 given the following inputs:
// - pagerank: the pagerank of the entity
// - nodes: the number of nodes in the graph
//
// Learn more: https://gist.github.com/pippellia-btc/8642a25fcf535edcda1ddecd0bcd5f7b
func Rank(pagerank float64, nodes int) int {
	b := 0.76
	a := 0.38  // curvature control
	C := 1 - b // base constant

	denom := float64(nodes)*pagerank + C
	value := 1 - math.Pow((C/denom), a)
	return int(math.Round(100 * value))
}

// Args returns the default RankProfilesArgs for the given pubkeys.
func Args(pubkeys []string) core.RankProfilesArgs {
	limit := min(len(pubkeys), core.RankProfilesLimit)

	return core.RankProfilesArgs{
		Algorithm: core.Algorithm{Sort: core.Global},
		Targets:   pubkeys[:limit],
		Limit:     limit,
	}
}
