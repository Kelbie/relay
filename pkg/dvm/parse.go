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
	// DVM parameters and defaults
	Global       string = "globalPagerank"
	Personalized string = "personalizedPagerank"
	ValidSorts          = []string{Global, Personalized}

	DefaultLimit int = 5
	MaxLimit     int = 1000
)

var (
	// parsing errors
	ErrNilEvent          error = errors.New("nil event pointer")
	ErrTooManyTags       error = errors.New("too many tags")
	ErrInvalidKind       error = errors.New("invalid kind: we only support kinds 5312 to 5315")
	ErrParamNotSupported error = errors.New("unsupported parameter")
	ErrMultipleParams    error = errors.New("too many parameters of the same type")

	// value errors
	ErrInvalidSource     error = errors.New("invalid source")
	ErrInvalidSort       error = errors.New(fmt.Sprintf("sort must be one of the following: %v", ValidSorts))
	ErrInvalidTarget     error = errors.New("invalid target")
	ErrInvalidLimit      error = errors.New("invalid limit")
	ErrInvalidSearch     error = errors.New("invalid search")
	ErrBadlyFormattedKey error = errors.New("badly formatted key")

	// internal system errors
	ErrInternal error = errors.New("internal error")
	ErrNilArgs  error = errors.New("nil args pointer")
)

// Algorithm is the sorting algorithm used in the DVM responses.
type Algorithm struct {
	Sort   string `json:"sort,omitempty"`
	Source string `json:"source,omitempty"`
}

type VerifyReputationArgs struct {
	Algorithm
	Target string
	Limit  int
}

type SortProfilesArgs struct {
	Algorithm
	Targets []string
	Limit   int
}

type RecommendFollowsArgs struct {
	Algorithm
	Limit int
}

type SearchProfilesArgs struct {
	Algorithm
	Search string
	Limit  int
}

// Params contains all the param fields of all DVMs.
// For each request method (DVM, REQ filter, http...), the [Parse] function should
// always return p Params, which will then be converted using the appropriate method.
// This way, adding a new request method will require writing only one parsing function.
type Params struct {
	Algorithm
	Targets []string `json:"targets,omitempty"`
	Search  string   `json:"search,omitempty"`
	Limit   int
}

// NewParams returns the default values for the params.
func NewParams(pubkey string) Params {
	return Params{
		Algorithm: Algorithm{Sort: Global, Source: pubkey},
		Limit:     DefaultLimit,
	}
}

func (p Params) ToVerifyReputationArgs() (*VerifyReputationArgs, error) {
	if len(p.Targets) != 1 {
		return nil, fmt.Errorf("%w: VerifyReputation requires exactly one 'target'", ErrInvalidTarget)
	}

	if len(p.Search) > 0 {
		return nil, fmt.Errorf("%w: VerifyReputation doesn't support 'search'", ErrParamNotSupported)
	}

	args := &VerifyReputationArgs{
		Algorithm: p.Algorithm,
		Target:    p.Targets[0],
		Limit:     p.Limit}

	if err := args.Normalize(); err != nil {
		return nil, err
	}

	return args, nil
}

func (p Params) ToRecommendFollowsArgs() (*RecommendFollowsArgs, error) {
	if len(p.Targets) > 0 {
		return nil, fmt.Errorf("%w: RecommendFollows doesn't support 'target'", ErrParamNotSupported)
	}

	if len(p.Search) > 0 {
		return nil, fmt.Errorf("%w: RecommendFollows doesn't support 'search'", ErrParamNotSupported)
	}

	args := &RecommendFollowsArgs{
		Algorithm: p.Algorithm,
		Limit:     p.Limit}

	if err := args.Normalize(); err != nil {
		return nil, err
	}

	return args, nil
}

func (p Params) ToSortProfilesArgs() (*SortProfilesArgs, error) {
	if len(p.Search) > 0 {
		return nil, fmt.Errorf("%w: SortProfiles doesn't support 'search'", ErrParamNotSupported)
	}

	args := &SortProfilesArgs{
		Algorithm: p.Algorithm,
		Targets:   p.Targets,
		Limit:     p.Limit}

	if err := args.Normalize(); err != nil {
		return nil, err
	}

	return args, nil
}

func (p Params) ToSearchProfilesArgs() (*SearchProfilesArgs, error) {
	if len(p.Targets) > 0 {
		return nil, fmt.Errorf("%w: SearchProfiles doesn't support 'target'", ErrParamNotSupported)
	}

	args := &SearchProfilesArgs{
		Algorithm: p.Algorithm,
		Search:    p.Search,
		Limit:     p.Limit}

	if err := args.Normalize(); err != nil {
		return nil, err
	}

	return args, nil
}

func (a *Algorithm) Normalize() error {
	if !slices.Contains(ValidSorts, a.Sort) {
		return fmt.Errorf("%w: %v", ErrInvalidSort, a.Sort)
	}

	if a.Sort == Personalized {
		var err error
		a.Source, err = ToHexPubkey(a.Source)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidSource, err)
		}
	}

	return nil
}

func (a *VerifyReputationArgs) Normalize() error {
	if a.Limit < 1 || a.Limit > MaxLimit {
		return fmt.Errorf("%w: limit must be an integer between 1 and %d: %d", ErrInvalidLimit, MaxLimit, a.Limit)
	}

	err := a.Algorithm.Normalize()
	if err != nil {
		return err
	}

	a.Target, err = ToHexPubkey(a.Target)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidTarget, err)
	}

	return nil
}

func (a *RecommendFollowsArgs) Normalize() error {
	if a.Limit < 1 || a.Limit > MaxLimit {
		return fmt.Errorf("%w: limit must be between 1 and %d: %d", ErrInvalidLimit, MaxLimit, a.Limit)
	}

	if err := a.Algorithm.Normalize(); err != nil {
		return err
	}

	return nil
}

func (a *SortProfilesArgs) Normalize() error {
	if a.Limit < 1 || a.Limit > MaxLimit {
		return fmt.Errorf("%w: limit must be between 1 and %d: %d", ErrInvalidLimit, MaxLimit, a.Limit)
	}

	err := a.Algorithm.Normalize()
	if err != nil {
		return err
	}

	if len(a.Targets) < 1 {
		return fmt.Errorf("%w: at least one target must be supplied for SortProfiles", ErrInvalidTarget)
	}

	for i, target := range a.Targets {
		a.Targets[i], err = ToHexPubkey(target)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidTarget, err)
		}
	}

	a.Limit = min(a.Limit, len(a.Targets))
	return nil
}

func (a *SearchProfilesArgs) Normalize() error {
	if a.Limit < 1 || a.Limit > MaxLimit {
		return fmt.Errorf("%w: limit must be between 1 and %d: %d", ErrInvalidLimit, MaxLimit, a.Limit)
	}

	if err := a.Algorithm.Normalize(); err != nil {
		return err
	}

	a.Search = strings.TrimSpace(a.Search)
	if len(a.Search) < 3 {
		return fmt.Errorf("%w: the search parameter must be longer than 3 characters (excluding leading and trailing spaces)", ErrInvalidSearch)
	}

	return nil
}

func Parse(req *nostr.Event) (Params, error) {
	params := NewParams(req.PubKey)
	counter := make(map[string]int, 5)

	if len(req.Tags) > MaxLimit+4 {
		return Params{}, ErrTooManyTags
	}

	for _, tag := range req.Tags {
		if len(tag) < 3 {
			continue
		}

		prefix, key, val := tag[0], tag[1], tag[2]
		if prefix != "param" {
			continue
		}

		counter[key]++
		switch key {

		case "target":
			params.Targets = append(params.Targets, val)

		case "source":
			params.Source = val

		case "sort":
			params.Sort = val

		case "search":
			params.Search = val

		case "limit":
			l, err := strconv.Atoi(val)
			if err != nil {
				return Params{}, fmt.Errorf("%w: limit must be an integer between 1 and %d: %s", ErrInvalidLimit, MaxLimit, val)
			}

			params.Limit = l

		default:
			return Params{}, fmt.Errorf("%w: param must be one between 'target', 'source', 'sort', 'search', 'limit' %v", ErrParamNotSupported, key)
		}
	}

	for key, val := range counter {
		if key != "target" && val > 1 {
			return Params{}, fmt.Errorf("%w: at most one '%s' can be provided", ErrMultipleParams, key)
		}
	}

	return params, nil
}

// ToHexPubkey() returns a parsed hex pubkey from the specified string.
func ToHexPubkey(key string) (string, error) {
	if nostr.IsValidPublicKey(key) {
		return key, nil
	}

	if strings.HasPrefix(key, "npub") {
		_, pubkey, err := nip19.Decode(key)
		if err != nil {
			return "", fmt.Errorf("%w: '%s'", ErrBadlyFormattedKey, key)
		}

		pk, ok := pubkey.(string)
		if !ok {
			return "", fmt.Errorf("%w: '%s'", ErrBadlyFormattedKey, key)
		}

		return pk, err
	}

	return "", fmt.Errorf("%w: '%s'", ErrBadlyFormattedKey, key)
}
