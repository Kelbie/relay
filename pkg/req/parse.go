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
	args := dvm.NewArgs()
	args.Kind = DVMkind - 1000

	if err = json.Unmarshal([]byte(filter.Search), &args); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnmarshalling, err)
	}

	// parse source key
	args.Source, err = dvm.ParseKey(args.Source)
	if err != nil {
		return nil, err
	}

	// parse targets in place
	for i, target := range args.Targets {
		t, err := dvm.ParseKey(target)
		if err != nil {
			return nil, err
		}

		args.Targets[i] = t
	}

	// validate sort, distance and limit.
	if !slices.Contains(dvm.ValidSorts, args.Sort) {
		return nil, fmt.Errorf("%w: %v", dvm.ErrInvalidSortOption, args.Sort)
	}

	if args.Distance > dvm.MaxDistance {
		return nil, fmt.Errorf("%w: distance must be smaller than %v", dvm.ErrInvalidDistance, dvm.MaxDistance)
	}

	if args.Limit > dvm.MaxLimit {
		return nil, fmt.Errorf("%w: limit must be smaller than %v", dvm.ErrInvalidLimit, dvm.MaxLimit)
	}

	return args, nil
}
