// The package req parses normal relay REQ into our dvm arguments. Specifically,
// the field filter.Search MUST be escaped JSON, and used to pass all parameters (e.g. "source", "target"...)
package req

import (
	"encoding/json"
	"errors"
	"fmt"
	"relay/pkg/dvm"
	"slices"

	"github.com/nbd-wtf/go-nostr"
)

var (
	ErrNilFilter          error = errors.New("nil filter pointer")
	ErrEmptyFieldSearch   error = errors.New("empty field search")
	ErrInvalidKindsFormat error = errors.New("the kinds of \"DVM-compatible\" filters must match this format: kinds:{<dvm_response_kind>, 7000}")
	ErrUnmarshalling      error = errors.New("error unmarshalling search field")
)

type OldArgs struct {
	// copied from the request event
	ID     string
	Pubkey string
	Kind   int

	Source  string   `json:"source,omitempty"`
	Targets []string `json:"targets,omitempty"`
	Sort    string   `json:"sort,omitempty"`
	Limit   uint64   `json:"limit,omitempty"`
	// Distance uint64   `json:"distance,omitempty"`
	// RequireProof    bool
}

// NewArgs() returns an Args struct with default arguments.
func NewOldArgs(ID, Pubkey string, Kind int) *OldArgs {
	return &OldArgs{
		ID:     ID,
		Kind:   Kind,
		Pubkey: Pubkey,

		Source: Pubkey,
		Sort:   dvm.DefaultSort,
		Limit:  dvm.DefaultLimit,
		// Distance: DefaultDistance,
	}
}

// Parse() parses a filter and returns the specified arguments for the DVM.
func Parse(filter *nostr.Filter) (*dvm.Args, error) {
	if filter == nil {
		return nil, ErrNilFilter
	}

	if filter.Search == "" {
		return nil, ErrEmptyFieldSearch
	}

	if len(filter.Kinds) != 2 {
		return nil, fmt.Errorf("%w :%v", ErrInvalidKindsFormat, filter.Kinds)
	}

	DVMkind, DVMerr := filter.Kinds[0], filter.Kinds[1]
	if DVMkind < 6312 || DVMkind > 6318 || DVMerr != 7000 {
		return nil, fmt.Errorf("%w :%v", ErrInvalidKindsFormat, filter.Kinds)
	}

	var err error
	var defaultArgs = dvm.NewArgs("", "", DVMkind-1000)
	var args = *defaultArgs

	// oldArgs is only used for unmarshalling, keeping compability with the old arguments structure
	var oldArgs = NewOldArgs("", "", DVMkind-1000)

	if err = json.Unmarshal([]byte(filter.Search), &oldArgs); err != nil {
		return defaultArgs, fmt.Errorf("%w: %v", ErrUnmarshalling, err)
	}

	// now we convert to the new argument structure
	args.Sources = []string{oldArgs.Source}
	args.Targets = oldArgs.Targets
	args.Sort = oldArgs.Sort
	args.Limit = oldArgs.Limit

	// parse source key
	args.Sources, err = dvm.ParseKeys(args.Sources)
	if err != nil {
		return defaultArgs, err
	}

	// parse targets
	args.Targets, err = dvm.ParseKeys(args.Targets)
	if err != nil {
		return defaultArgs, err
	}

	// validate sort, distance and limit.
	if !slices.Contains(dvm.ValidSorts, args.Sort) {
		return defaultArgs, fmt.Errorf("%w: %v", dvm.ErrInvalidSortOption, args.Sort)
	}

	if args.Limit > dvm.MaxLimit {
		return defaultArgs, fmt.Errorf("%w: limit must be smaller than %v", dvm.ErrInvalidLimit, dvm.MaxLimit)
	}

	// if args.Distance > dvm.MaxDistance {
	// 	return defaultArgs, fmt.Errorf("%w: distance must be smaller than %v", dvm.ErrInvalidDistance, dvm.MaxDistance)
	// }

	return &args, nil
}
