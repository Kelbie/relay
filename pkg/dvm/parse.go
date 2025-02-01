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
	ErrInvalidKind       error = errors.New("invalid kind; we support kinds 5312 to 5318")
	ErrBadlyFormattedTag error = errors.New("tag should be 'param, <key>, <vals>'")
	ErrUnknownParameter  error = errors.New("parameter must be one between 'sources', 'targets', 'sort', 'limit'")
	ErrEmptyKeys         error = errors.New("at least one key must be specified")
	ErrBadlyFormattedKey error = errors.New("badly formatted key")
	ErrBadlyFormattedInt error = errors.New("badly formatted unsigned integer")

	// value errors
	ErrInvalidSortOption error = errors.New("sort must be one between 'global', 'personalized'")
	ErrInvalidSources    error = errors.New("invalid sources")
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

	Sources []string
	Targets []string
	Sort    string
	Limit   uint64
	// Distance uint64   `json:"distance,omitempty"`
	// RequireProof    bool
}

// NewArgs() returns an Args struct with default arguments.
func NewArgs(ID, Pubkey string, Kind int) *Args {
	return &Args{
		ID:     ID,
		Kind:   Kind,
		Pubkey: Pubkey,

		Sources: []string{Pubkey},
		Sort:    DefaultSort,
		Limit:   DefaultLimit,
		// Distance: DefaultDistance,
	}
}

// Parse() parses and returns the arguments of the request event as an Args struct.
// In case of any error, the default arguments are returned in order for ErrorEvent() to have req.pubkey and req.ID
func Parse(req *nostr.Event) (*Args, error) {
	if req == nil {
		return nil, ErrNilEvent
	}

	if req.Kind < 5312 || req.Kind > 5318 {
		return nil, fmt.Errorf("%w: %v", ErrInvalidKind, req.Kind)
	}

	var defaultArgs = NewArgs(req.ID, req.PubKey, req.Kind)
	var args = *defaultArgs // this copy will be returned if no errors occur.

	for _, tag := range req.Tags {
		if len(tag) < 3 {
			return defaultArgs, fmt.Errorf("%w: %v", ErrBadlyFormattedTag, tag)
		}

		prefix, key, vals := tag[0], tag[1], tag[2:]
		if prefix != "param" {
			return defaultArgs, fmt.Errorf("%w: %v", ErrBadlyFormattedTag, tag)
		}

		switch key {
		case "sources":
			if len(vals) == 0 {
				return defaultArgs, ErrEmptyKeys
			}

			keys, err := ParseKeys(vals)
			if err != nil {
				return defaultArgs, err
			}
			args.Sources = keys

		case "targets":
			if len(vals) == 0 {
				return defaultArgs, ErrEmptyKeys
			}

			keys, err := ParseKeys(vals)
			if err != nil {
				return defaultArgs, err
			}
			args.Targets = keys

		case "sort":
			if !slices.Contains(ValidSorts, vals[0]) {
				return defaultArgs, fmt.Errorf("%w: %v", ErrInvalidSortOption, vals[0])
			}
			args.Sort = vals[0]

		case "limit":
			l, err := strconv.ParseUint(vals[0], 10, 32)
			if err != nil {
				return defaultArgs, fmt.Errorf("%w: limit = %v", ErrBadlyFormattedInt, vals[0])
			}

			if l > MaxLimit {
				return defaultArgs, fmt.Errorf("%w: limit must be smaller than %v", ErrInvalidLimit, MaxLimit)
			}
			args.Limit = l

		// case "distance":
		// 	d, err := strconv.ParseUint(vals[0], 10, 32)
		// 	if err != nil {
		// 		return defaultArgs, fmt.Errorf("%w: distance = %v", ErrBadlyFormattedInt, vals[0])
		// 	}

		// 	if d > MaxDistance {
		// 		return defaultArgs, fmt.Errorf("%w: distance must be smaller than %v", ErrInvalidDistance, MaxDistance)
		// 	}
		// 	args.Distance = d

		default:
			return defaultArgs, fmt.Errorf("%w: got %v", ErrUnknownParameter, key)
		}
	}

	return &args, nil
}

// ParseKeys() returns a slice of parsed hex keys from the specified keys.
func ParseKeys(keys []string) ([]string, error) {

	// parse keys and update them in place
	for i, key := range keys {
		key, err := ParseKey(key)
		if err != nil {
			return nil, err
		}

		keys[i] = key
	}

	return keys, nil
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
