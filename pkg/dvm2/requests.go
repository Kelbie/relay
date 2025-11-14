package dvm2

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/nbd-wtf/go-nostr"
	"github.com/vertex-lab/relay/pkg/service"
)

const (
	KindVerifyReputation int = 5312
	KindRecommendFollows int = 5313
	KindRankProfiles     int = 5314
	KindSearchProfiles   int = 5315
	KindDVMError         int = 7000
)

var (
	ErrInvalidKind     = errors.New("invalid kind")
	ErrUnsupportedKind = errors.New("unsupported kind: we only support kinds 5312 to 5315")
)

func parseVerifyReputation(e *nostr.Event) (args service.VerifyReputationArgs, err error) {
	args = service.NewVerifyReputationArgs(e.PubKey)
	if e.Kind != KindVerifyReputation {
		return args, fmt.Errorf("%w: parseVerifyReputation got event kind %d", ErrInvalidKind, e.Kind)
	}

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
				return args, fmt.Errorf("%w: limit must be an integer: %s", service.ErrInvalidLimit, val)
			}

		default:
			return args, fmt.Errorf("%w: %v", service.ErrUnsuportedVerifyReputation, key)
		}
	}
	return args, nil
}

func parseRecommendFollows(e *nostr.Event) (args service.RecommendFollowsArgs, err error) {
	args = service.NewRecommendFollowsArgs(e.PubKey)
	if e.Kind != KindRecommendFollows {
		return args, fmt.Errorf("%w: parseRecommendFollows got event kind %d", ErrInvalidKind, e.Kind)
	}

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
				return args, fmt.Errorf("%w: limit must be an integer: %s", service.ErrInvalidLimit, val)
			}

		default:
			return args, fmt.Errorf("%w: %v", service.ErrUnsuportedRecommendFollows, key)
		}
	}
	return args, nil
}

func parseRankProfiles(e *nostr.Event) (args service.RankProfilesArgs, err error) {
	args = service.NewRankProfilesArgs(e.PubKey)
	if e.Kind != KindRankProfiles {
		return args, fmt.Errorf("%w: parseRankProfiles got event kind %d", ErrInvalidKind, e.Kind)
	}

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
				return args, fmt.Errorf("%w: limit must be an integer: %s", service.ErrInvalidLimit, val)
			}

		default:
			return args, fmt.Errorf("%w: %v", service.ErrUnsuportedRankProfiles, key)
		}
	}
	return args, nil
}

func parseSearchProfiles(e *nostr.Event) (args service.SearchProfilesArgs, err error) {
	args = service.NewSearchProfilesArgs(e.PubKey)
	if e.Kind != KindSearchProfiles {
		return args, fmt.Errorf("%w: parseSearchProfiles got event kind %d", ErrInvalidKind, e.Kind)
	}

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
				return args, fmt.Errorf("%w: limit must be an integer: %s", service.ErrInvalidLimit, val)
			}

		default:
			return args, fmt.Errorf("%w: %v", service.ErrUnsuportedRankProfiles, key)
		}
	}
	return args, nil
}
