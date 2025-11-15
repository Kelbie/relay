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
	ErrUnsupportedKind = errors.New("unsupported kind: we only support kinds 5312 to 5315")
)

// Parse a dvm request into one of the [service.Args].
func Parse(e *nostr.Event) (service.Args, error) {
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

func parseVerifyReputation(e *nostr.Event) (*service.VerifyReputationArgs, error) {
	args := service.NewVerifyReputationArgs(e.PubKey)
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
				return nil, fmt.Errorf("%w: limit must be an integer: %s", service.ErrInvalidLimit, val)
			}

		default:
			return nil, fmt.Errorf("%w: %v", service.ErrUnsuportedVerifyReputation, key)
		}
	}
	return &args, nil
}

func parseRecommendFollows(e *nostr.Event) (*service.RecommendFollowsArgs, error) {
	args := service.NewRecommendFollowsArgs(e.PubKey)
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
				return nil, fmt.Errorf("%w: limit must be an integer: %s", service.ErrInvalidLimit, val)
			}

		default:
			return nil, fmt.Errorf("%w: %v", service.ErrUnsuportedRecommendFollows, key)
		}
	}
	return &args, nil
}

func parseRankProfiles(e *nostr.Event) (*service.RankProfilesArgs, error) {
	args := service.NewRankProfilesArgs(e.PubKey)
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
				return nil, fmt.Errorf("%w: limit must be an integer: %s", service.ErrInvalidLimit, val)
			}

		default:
			return nil, fmt.Errorf("%w: %v", service.ErrUnsuportedRankProfiles, key)
		}
	}
	return &args, nil
}

func parseSearchProfiles(e *nostr.Event) (*service.SearchProfilesArgs, error) {
	args := service.NewSearchProfilesArgs(e.PubKey)
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
				return nil, fmt.Errorf("%w: limit must be an integer: %s", service.ErrInvalidLimit, val)
			}

		default:
			return nil, fmt.Errorf("%w: %v", service.ErrUnsuportedRankProfiles, key)
		}
	}
	return &args, nil
}
