package dvm

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/vertex-lab/crawler/pkg/utils/sliceutils"
)

// DVM parameters and defaults
var (
	ValidSorts          = []string{Global, Personalized, Followers}
	Global       string = "globalPagerank"
	Personalized string = "personalizedPagerank"
	Followers    string = "followerCount"

	DefaultLimit     int = 5
	StandardMaxLimit int = 100
	ExtendedMaxLimit int = 1000
)

var (
	// parsing errors
	ErrInvalidKind       error = errors.New("invalid kind: we only support kinds 5312 to 5315")
	ErrTooManyTags       error = errors.New("too many tags")
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

	// payment errors
	ErrNoCredits error = errors.New("you don't have enough credits to fulfil the request. Send us a DM and we'll top you up for free.")
)

// Algorithm is the sorting algorithm used in the DVM responses.
type Algorithm struct {
	Sort   string `json:"sort,omitempty"`
	Source string `json:"source,omitempty"`
}

// Request is the internal representation of the DVM request nostr.Event.
// For each request method (DVM, REQ filter, http...), the [Parse] function should
// always return a [Request], which will then be converted using the appropriate
// method To<argument's name>. This way, adding a new request method will require
// writing just one parsing function.
type Request struct {
	Record
	Algorithm
	Targets []string `json:"targets,omitempty"`
	Search  string   `json:"search,omitempty"`
	Limit   int
}

// NewRequest returns the default values for the Request.
func NewRequest(rec Record) Request {
	return Request{
		Record:    rec,
		Algorithm: Algorithm{Sort: Global, Source: rec.Pubkey},
		Limit:     DefaultLimit,
	}
}

func (r Request) ToTags() nostr.Tags {
	tags := r.Record.ToTags()
	tags = append(tags, nostr.Tag{"sort", r.Sort})

	if r.IsPersonalized() {
		tags = append(tags, nostr.Tag{"source", r.Source})
	}

	tags = append(tags, nostr.Tag{"nodes", strconv.Itoa(r.Nodes)})
	return tags
}

// IsPersonalized returns whether the request is personalized to the source pubkey.
func (r Request) IsPersonalized() bool {
	return r.Sort == Personalized || r.Record.Kind == KindRecommendFollows
}

// Record contains metadata about the request event.
// It also contains a snapshot of relevant system data at the time the request was received.
type Record struct {
	ID        string
	Pubkey    string
	Kind      int
	Nodes     int // the number of nodes in the graph when the request was received
	Timestamp nostr.Timestamp
}

func NewRecord(req *nostr.Event) Record {
	return Record{
		ID:        req.ID,
		Pubkey:    req.PubKey,
		Kind:      req.Kind,
		Timestamp: nostr.Now(),
	}
}

func (r Record) ToTags() nostr.Tags {
	return nostr.Tags{{"e", r.ID}, {"p", r.Pubkey}}
}

// Parse() parses all the tags with prefix "param" into a Request structure.
// If some params are not provided, the default values will be used.
func Parse(req *nostr.Event) (*Request, error) {
	if len(req.Tags) > ExtendedMaxLimit+4 {
		return nil, ErrTooManyTags
	}

	record := NewRecord(req)
	request := NewRequest(record)
	counter := make(map[string]int, 5)

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
			request.Targets = append(request.Targets, val)

		case "source":
			request.Source = val

		case "sort":
			request.Sort = val

		case "search":
			request.Search = val

		case "limit":
			l, err := strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("%w: limit must be an integer: %s", ErrInvalidLimit, val)
			}

			request.Limit = l

		default:
			return nil, fmt.Errorf("%w: param must be one between 'target', 'source', 'sort', 'search', 'limit' %v", ErrParamNotSupported, key)
		}
	}

	for key, val := range counter {
		if key != "target" && val > 1 {
			return nil, fmt.Errorf("%w: at most one '%s' can be provided", ErrMultipleParams, key)
		}
	}

	return &request, nil
}

type VerifyReputationArgs struct {
	Algorithm
	Target string
	Limit  int
}

type RankProfilesArgs struct {
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

func (r *Request) ToVerifyReputationArgs() (*VerifyReputationArgs, error) {
	if len(r.Targets) != 1 {
		return nil, fmt.Errorf("%w: VerifyReputation requires exactly one 'target'", ErrInvalidTarget)
	}

	if r.Search != "" {
		return nil, fmt.Errorf("%w: VerifyReputation doesn't support 'search'", ErrParamNotSupported)
	}

	args := &VerifyReputationArgs{
		Algorithm: r.Algorithm,
		Target:    r.Targets[0],
		Limit:     r.Limit}

	if err := args.Normalize(); err != nil {
		return nil, err
	}

	return args, nil
}

func (r *Request) ToRecommendFollowsArgs() (*RecommendFollowsArgs, error) {
	if len(r.Targets) > 0 {
		return nil, fmt.Errorf("%w: RecommendFollows doesn't support 'target'", ErrParamNotSupported)
	}

	if r.Search != "" {
		return nil, fmt.Errorf("%w: RecommendFollows doesn't support 'search'", ErrParamNotSupported)
	}

	args := &RecommendFollowsArgs{
		Algorithm: r.Algorithm,
		Limit:     r.Limit}

	if err := args.Normalize(); err != nil {
		return nil, err
	}

	return args, nil
}

func (r *Request) ToRankProfilesArgs() (*RankProfilesArgs, error) {
	if r.Search != "" {
		return nil, fmt.Errorf("%w: RankProfiles doesn't support 'search'", ErrParamNotSupported)
	}

	args := &RankProfilesArgs{
		Algorithm: r.Algorithm,
		Targets:   r.Targets,
		Limit:     r.Limit}

	if err := args.Normalize(); err != nil {
		return nil, err
	}

	return args, nil
}

func (r *Request) ToSearchProfilesArgs() (*SearchProfilesArgs, error) {
	if len(r.Targets) > 0 {
		return nil, fmt.Errorf("%w: SearchProfiles doesn't support 'target'", ErrParamNotSupported)
	}

	args := &SearchProfilesArgs{
		Algorithm: r.Algorithm,
		Search:    r.Search,
		Limit:     r.Limit}

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
	if a.Limit < 1 || a.Limit > StandardMaxLimit {
		return fmt.Errorf("%w: limit must be an integer between 1 and %d: %d", ErrInvalidLimit, StandardMaxLimit, a.Limit)
	}

	if err := a.Algorithm.Normalize(); err != nil {
		return err
	}

	var err error
	a.Target, err = ToHexPubkey(a.Target)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidTarget, err)
	}

	return nil
}

func (a *RecommendFollowsArgs) Normalize() error {
	if a.Limit < 1 || a.Limit > StandardMaxLimit {
		return fmt.Errorf("%w: limit must be between 1 and %d: %d", ErrInvalidLimit, StandardMaxLimit, a.Limit)
	}

	if err := a.Algorithm.Normalize(); err != nil {
		return err
	}

	return nil
}

func (a *RankProfilesArgs) Normalize() error {
	if a.Limit < 1 || a.Limit > ExtendedMaxLimit {
		return fmt.Errorf("%w: limit must be between 1 and %d: %d", ErrInvalidLimit, ExtendedMaxLimit, a.Limit)
	}

	a.Targets = sliceutils.Unique(a.Targets)
	if len(a.Targets) < 1 || len(a.Targets) > ExtendedMaxLimit {
		return fmt.Errorf("%w: the number of targets must be between 1 and %d: %d", ErrInvalidTarget, ExtendedMaxLimit, len(a.Targets))
	}

	a.Limit = min(a.Limit, len(a.Targets))
	if err := a.Algorithm.Normalize(); err != nil {
		return err
	}

	for i, target := range a.Targets {
		pk, err := ToHexPubkey(target)
		if err != nil {
			// do nothing on invalid pubkeys; they will simply receive a rank of 0.
			continue
		}

		if pk != target {
			a.Targets[i] = pk
		}
	}
	return nil
}

func (a *SearchProfilesArgs) Normalize() error {
	if a.Limit < 1 || a.Limit > StandardMaxLimit {
		return fmt.Errorf("%w: limit must be between 1 and %d: %d", ErrInvalidLimit, StandardMaxLimit, a.Limit)
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

// ToHexPubkey() returns a parsed hex pubkey from the specified string.
func ToHexPubkey(key string) (string, error) {
	key = strings.TrimSpace(key)
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

		return pk, nil
	}

	return "", fmt.Errorf("%w: '%s'", ErrBadlyFormattedKey, key)
}
