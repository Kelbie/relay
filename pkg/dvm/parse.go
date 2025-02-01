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
	ErrNilEvent          error = errors.New("nil event pointer")
	ErrInvalidKind       error = errors.New("invalid kind; we support kinds 5312 to 5314")
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
	// copied from the request event
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
func NewArgs(ID, Pubkey string, Kind int) *Args {
	return &Args{
		ID:     ID,
		Kind:   Kind,
		Pubkey: Pubkey,

		Source:   Pubkey,
		Sort:     DefaultSort,
		Distance: DefaultDistance,
		Limit:    DefaultLimit,
	}
}

// Parse() parses and returns the arguments of the request event as an Args struct.
// In case of any error, the default arguments are returned in order for ErrorEvent() to have req.pubkey and req.ID
func Parse(req *nostr.Event) (*Args, error) {
	if req == nil {
		return nil, ErrNilEvent
	}

	if req.Kind < 5312 || req.Kind > 5314 {
		return nil, fmt.Errorf("%w: %v", ErrInvalidKind, req.Kind)
	}

	var defaultArgs = NewArgs(req.ID, req.PubKey, req.Kind)
	var args = *defaultArgs // this copy will be returned if no errors occur.

	for _, tag := range req.Tags {
		if len(tag) < 3 {
			return defaultArgs, fmt.Errorf("%w: %v", ErrBadlyFormattedTag, tag)
		}

		prefix, key, val := tag[0], tag[1], tag[2]
		if prefix != "param" {
			return defaultArgs, fmt.Errorf("%w: %v", ErrBadlyFormattedTag, tag)
		}

		switch key {
		case "source":
			pk, err := ParseKey(val)
			if err != nil {
				return defaultArgs, err
			}
			args.Source = pk

		case "target":
			pk, err := ParseKey(val)
			if err != nil {
				return defaultArgs, err
			}
			args.Targets = append(args.Targets, pk)

		case "sort":
			if !slices.Contains(ValidSorts, val) {
				return defaultArgs, fmt.Errorf("%w: %v", ErrInvalidSortOption, val)
			}
			args.Sort = val

		case "distance":
			d, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return defaultArgs, fmt.Errorf("%w: distance = %v", ErrBadlyFormattedInt, val)
			}

			if d > MaxDistance {
				return defaultArgs, fmt.Errorf("%w: distance must be smaller than %v", ErrInvalidDistance, MaxDistance)
			}
			args.Distance = d

		case "limit":
			l, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return defaultArgs, fmt.Errorf("%w: limit = %v", ErrBadlyFormattedInt, val)
			}

			if l > MaxLimit {
				return defaultArgs, fmt.Errorf("%w: limit must be smaller than %v", ErrInvalidLimit, MaxLimit)
			}
			args.Limit = l

		default:
			return defaultArgs, fmt.Errorf("%w: got %v", ErrUnknownParameter, key)
		}
	}

	return &args, nil
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
