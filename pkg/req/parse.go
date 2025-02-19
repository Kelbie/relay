// The package req parses normal relay REQ into our dvm arguments. Specifically,
// the field filter.Search MUST be escaped JSON, and used to pass all parameters (e.g. "source", "target"...)
package req

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/vertex-lab/relay/pkg/dvm"

	"github.com/nbd-wtf/go-nostr"
)

var (
	ErrNilFilter          error = errors.New("nil filter pointer")
	ErrEmptyFieldSearch   error = errors.New("empty field search")
	ErrInvalidKindsFormat error = errors.New("the kinds of \"DVM-compatible\" filters must match this format: kinds:{<dvm_response_kind>, 7000}")
	ErrUnmarshalling      error = errors.New("error unmarshalling search field")
)

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
	var args = *defaultArgs // this copy will be returned if no errors occur.

	if err = json.Unmarshal([]byte(filter.Search), &args); err != nil {
		return defaultArgs, fmt.Errorf("%w: %v", ErrUnmarshalling, err)
	}

	// parse source key if provided
	if args.Source != "" {
		args.Source, err = dvm.ParseKey(args.Source)
		if err != nil {
			return defaultArgs, err
		}
	}

	// parse targets in place
	for i, target := range args.Targets {
		t, err := dvm.ParseKey(target)
		if err != nil {
			return defaultArgs, err
		}

		args.Targets[i] = t
	}

	// validate sort, distance and limit.
	if !slices.Contains(dvm.ValidSorts, args.Sort) {
		return defaultArgs, fmt.Errorf("%w: %v", dvm.ErrInvalidSortOption, args.Sort)
	}

	if args.Limit > dvm.MaxLimit {
		return defaultArgs, fmt.Errorf("%w: limit must be smaller than %v", dvm.ErrInvalidLimit, dvm.MaxLimit)
	}

	return &args, nil
}
