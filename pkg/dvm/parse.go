package dvm

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

var (
	DefaultSort string = "global"
	ValidSorts         = []string{"personalized", "global"}

	DefaultDistance uint64 = 0 // meaning no constrain on the distance
	MaxDistance     uint64 = 5
	DefaultLimit    uint64 = 5
	MaxLimit        uint64 = 1000
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

// The Args structure contains the general input parameters for our service.
type Args struct {
	// same as the equivalent from request
	ID     string
	Pubkey string
	Kind   int

	Source   string   `json:"source,omitempty"`
	Targets  []string `json:"targets,omitempty"`
	Sort     string   `json:"sort,omitempty"`
	Distance uint64   `json:"distance,omitempty"`
	Limit    uint64   `json:"limit,omitempty"`
	// RequireProof    bool
}

// NewArgs() returns an Args struct with default arguments.
func NewArgs() *Args {
	return &Args{
		Sort:     DefaultSort,
		Distance: DefaultDistance,
		Limit:    DefaultLimit,
	}
}

// Parse() parses and returns the arguments of the request event as an Args struct.
func Parse(req *nostr.Event) (*Args, error) {
	if req == nil {
		return nil, ErrNilRequest
	}

	args := NewArgs()
	args.ID = req.ID
	args.Kind = req.Kind
	args.Pubkey, args.Source = req.PubKey, req.PubKey

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
			if !slices.Contains(ValidSorts, val) {
				return nil, fmt.Errorf("%w: %v", ErrInvalidSortOption, val)
			}
			args.Sort = val

		case "distance":
			d, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("%w: distance = %v", ErrBadlyFormattedInt, val)
			}

			if d > MaxDistance {
				return nil, fmt.Errorf("%w: distance must be smaller than %v", ErrInvalidDistance, MaxDistance)
			}
			args.Distance = d

		case "limit":
			l, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("%w: limit = %v", ErrBadlyFormattedInt, val)
			}

			if l > MaxLimit {
				return nil, fmt.Errorf("%w: limit must be smaller than %v", ErrInvalidLimit, MaxLimit)
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
