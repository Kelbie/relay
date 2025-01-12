package dvm

import (
	"fmt"
	"relay/pkg/response"
	"slices"
	"strconv"
	"strings"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

// ParseArgs() parses and returns the arguments of the request event as an Args struct.
func ParseArgs(req *nostr.Event) (*response.Args, error) {
	if req == nil {
		return nil, response.ErrNilRequest
	}

	args := response.NewArgs(req.PubKey)
	for _, tag := range req.Tags {
		if len(tag) < 3 {
			return nil, fmt.Errorf("%w: %v", response.ErrBadlyFormattedTag, tag)
		}

		prefix, key, val := tag[0], tag[1], tag[2]
		if prefix != "param" {
			return nil, fmt.Errorf("%w: %v", response.ErrBadlyFormattedTag, tag)
		}

		switch key {
		case "source":
			pk, err := ParseKey(val)
			if err != nil {
				return nil, err
			}
			args.Source = pk

		case "target":
			pk, err := ParseKey(val)
			if err != nil {
				return nil, err
			}
			args.Targets = append(args.Targets, pk)

		case "sort":
			if !slices.Contains(response.ValidSorts, val) {
				return nil, fmt.Errorf("%w: %v", response.ErrInvalidSortOption, val)
			}
			args.Sort = val

		case "distance":
			d, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("%w: distance = %v", response.ErrBadlyFormattedInt, val)
			}

			if d > response.MaxDistance {
				return nil, fmt.Errorf("%w: distance must be smaller than %v", response.ErrInvalidDistance, response.MaxDistance)
			}
			args.Distance = d

		case "limit":
			l, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("%w: limit = %v", response.ErrBadlyFormattedInt, val)
			}

			if l > response.MaxLimit {
				return nil, fmt.Errorf("%w: limit must be smaller than %v", response.ErrInvalidLimit, response.MaxLimit)
			}
			args.Limit = l

		default:
			return nil, fmt.Errorf("%w: got %v", response.ErrUnknownParameter, key)
		}
	}

	return args, nil
}

// ParseKey() returns a parsed hex key from the specified string.
func ParseKey(key string) (string, error) {
	if nostr.IsValidPublicKey(key) {
		return key, nil
	}

	if strings.HasPrefix(key, "npub") {
		_, pubkey, err := nip19.Decode(key)
		if err != nil {
			return "", fmt.Errorf("%w: %v", response.ErrBadlyFormattedKey, key)
		}

		pk, ok := pubkey.(string)
		if !ok {
			return "", fmt.Errorf("%w: %v", response.ErrBadlyFormattedKey, key)
		}

		return pk, err
	}

	return "", fmt.Errorf("%w: %v", response.ErrBadlyFormattedKey, key)
}
