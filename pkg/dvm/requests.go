// The package dvm is responsible for defining the dvm-specific logic for parsing
// requests, and encoding responses as nostr events.
package dvm

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/nbd-wtf/go-nostr"
	"github.com/vertex-lab/relay/pkg/core"
)

const (
	KindVerifyReputation int = 5312
	KindRecommendFollows int = 5313
	KindRankProfiles     int = 5314
	KindSearchProfiles   int = 5315
	KindDVMError         int = 7000
)

var (
	ErrUnsupportedKind = errors.New("unsupported kind: we only support kinds 5312 to 5315")
)

// Parse a dvm request into one of the [core.Args].
// It doesn't checks for ID or signature.
func Parse(e *nostr.Event) (core.Args, error) {
	switch e.Kind {
	case KindVerifyReputation:
		return parseVerifyReputation(e)

	case KindRecommendFollows:
		return parseRecommendFollows(e)

	case KindRankProfiles:
		return parseRankProfiles(e)

	case KindSearchProfiles:
		return parseSearchProfiles(e)

	default:
		return nil, fmt.Errorf("%w: %d", ErrUnsupportedKind, e.Kind)
	}
}

func Supports(kind int) bool {
	return KindVerifyReputation <= kind && kind <= KindSearchProfiles
}

func parseVerifyReputation(e *nostr.Event) (*core.VerifyReputationArgs, error) {
	args := core.NewVerifyReputationArgs(e.PubKey)
	var err error

	for _, tag := range e.Tags {
		if len(tag) < 3 {
			continue
		}

		prefix, key, val := tag[0], tag[1], tag[2]
		if prefix != "param" {
			continue
		}

		switch key {
		case "target":
			args.Target = val

		case "source":
			args.Source = val

		case "sort":
			args.Sort = val

		case "limit":
			args.Limit, err = strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("%w: limit must be an integer: %s", core.ErrInvalidLimit, val)
			}

		default:
			return nil, fmt.Errorf("%w: %v", core.ErrUnsuportedVerifyReputation, key)
		}
	}
	return &args, nil
}

func parseRecommendFollows(e *nostr.Event) (*core.RecommendFollowsArgs, error) {
	args := core.NewRecommendFollowsArgs(e.PubKey)
	var err error

	for _, tag := range e.Tags {
		if len(tag) < 3 {
			continue
		}

		prefix, key, val := tag[0], tag[1], tag[2]
		if prefix != "param" {
			continue
		}

		switch key {
		case "source":
			args.Source = val

		case "sort":
			args.Sort = val

		case "limit":
			args.Limit, err = strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("%w: limit must be an integer: %s", core.ErrInvalidLimit, val)
			}

		default:
			return nil, fmt.Errorf("%w: %v", core.ErrUnsuportedRecommendFollows, key)
		}
	}
	return &args, nil
}

func parseRankProfiles(e *nostr.Event) (*core.RankProfilesArgs, error) {
	args := core.NewRankProfilesArgs(e.PubKey)
	var err error

	for _, tag := range e.Tags {
		if len(tag) < 3 {
			continue
		}

		prefix, key, val := tag[0], tag[1], tag[2]
		if prefix != "param" {
			continue
		}

		switch key {
		case "target":
			args.Targets = append(args.Targets, val)

		case "source":
			args.Source = val

		case "sort":
			args.Sort = val

		case "limit":
			args.Limit, err = strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("%w: limit must be an integer: %s", core.ErrInvalidLimit, val)
			}

		default:
			return nil, fmt.Errorf("%w: %v", core.ErrUnsuportedRankProfiles, key)
		}
	}
	return &args, nil
}

func parseSearchProfiles(e *nostr.Event) (*core.SearchProfilesArgs, error) {
	args := core.NewSearchProfilesArgs(e.PubKey)
	var err error

	for _, tag := range e.Tags {
		if len(tag) < 3 {
			continue
		}

		prefix, key, val := tag[0], tag[1], tag[2]
		if prefix != "param" {
			continue
		}

		switch key {
		case "source":
			args.Source = val

		case "sort":
			args.Sort = val

		case "search":
			args.Search = val

		case "limit":
			args.Limit, err = strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("%w: limit must be an integer: %s", core.ErrInvalidLimit, val)
			}

		default:
			return nil, fmt.Errorf("%w: %v", core.ErrUnsuportedRankProfiles, key)
		}
	}
	return &args, nil
}
