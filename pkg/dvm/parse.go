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

var validSorts = []string{"personalized", "global"}

const (
	defaultDistance int    = -1 // meaning no constrain on the distance
	defaultLimit    int    = 5
	defaultSort     string = "global"
)

// The Args structure contains the general input parameters for our DVMs.
type Args struct {
	Source   string
	Targets  []string
	Sort     string
	Distance int
	Limit    int
	// PequireProof    bool		better to leave it for the future
}

// NewArgs() returns the default arguments.
func NewArgs(pubkey string) *Args {
	return &Args{
		Source:   pubkey,
		Targets:  []string{},
		Distance: defaultDistance,
		Sort:     defaultSort,
		Limit:    defaultLimit,
	}
}

// ParseArgs() returns the arguments of the request event as an Args struct.
func ParseArgs(req *nostr.Event) (*Args, error) {
	if req == nil {
		return nil, nil
	}

	args := NewArgs(req.PubKey)
	for _, tag := range req.Tags {
		if len(tag) < 3 {
			return nil, fmt.Errorf("%w: %v", ErrBadlyFormattedTag, tag)
		}

		if tag[0] != "param" {
			return nil, fmt.Errorf("%w: %v", ErrBadlyFormattedTag, tag)
		}

		prefix, value := tag[1], tag[2]
		switch prefix {
		case "source":
			pk, err := ParseKey(value)
			if err != nil {
				return nil, err
			}
			args.Source = pk

		case "target":
			pk, err := ParseKey(value)
			if err != nil {
				return nil, err
			}
			args.Targets = append(args.Targets, pk)

		case "sort":
			if !slices.Contains(validSorts, value) {
				return nil, ErrInvalidSortOption
			}
			args.Sort = value

		case "distance":
			d, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("%w: distance = %v", ErrBadlyFormattedInt, value)
			}
			args.Distance = d

		case "limit":
			l, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("%w: limit = %v", ErrBadlyFormattedInt, value)
			}
			args.Limit = l
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

// ---------------------------------ERROR-CODES--------------------------------

var ErrBadlyFormattedTag error = errors.New("tag should be 'param, <prefix>, <value>'")
var ErrBadlyFormattedKey error = errors.New("badly formatted key")
var ErrBadlyFormattedInt error = errors.New("badly formatted integer")
var ErrInvalidSortOption error = errors.New("sort must be one between 'global', 'personalized'")
