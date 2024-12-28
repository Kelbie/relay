package dvm

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

var (
	defaultDistance uint64 = 0 // meaning no constrain on the distance
	defaultLimit    uint64 = 5
	defaultSort     string = "global"
	validSorts             = []string{"personalized", "global"}
)

// The Args structure contains the general input parameters for our DVMs.
type Args struct {
	Source   string
	Targets  []string
	Sort     string
	Distance uint64
	Limit    uint64
	// RequireProof    bool		better to leave it for the future
}

// NewArgs() returns an Args struct with default arguments.
func NewArgs(pubkey string) *Args {
	return &Args{
		Source:   pubkey,
		Targets:  []string{},
		Sort:     defaultSort,
		Distance: defaultDistance,
		Limit:    defaultLimit,
	}
}

// ParseArgs() parses and returns the arguments of the request event as an Args struct.
func ParseArgs(req *nostr.Event) (*Args, error) {
	if req == nil {
		return nil, nil
	}

	args := NewArgs(req.PubKey)
	for _, tag := range req.Tags {
		if len(tag) < 3 {
			return nil, fmt.Errorf("%w: %v", ErrBadlyFormattedTag, tag)
		}

		prefix, key, val := tag[0], tag[1], tag[2]

		if prefix != "param" {
			return nil, fmt.Errorf("%w: %v", ErrBadlyFormattedTag, tag)
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
			if !slices.Contains(validSorts, val) {
				return nil, ErrInvalidSortOption
			}
			args.Sort = val

		case "distance":
			d, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("%w: distance = %v", ErrBadlyFormattedInt, val)
			}
			args.Distance = d

		case "limit":
			l, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("%w: limit = %v", ErrBadlyFormattedInt, val)
			}
			args.Limit = l

		default:
			return nil, fmt.Errorf("%w: got %v", ErrUnknownParameter, key)
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
			return "", fmt.Errorf("%w: %v", ErrBadlyFormattedKey, key)
		}

		pk, ok := pubkey.(string)
		if !ok {
			return "", fmt.Errorf("%w: %v", ErrBadlyFormattedKey, key)
		}

		return pk, err
	}

	return "", fmt.Errorf("%w: %v", ErrBadlyFormattedKey, key)
}
