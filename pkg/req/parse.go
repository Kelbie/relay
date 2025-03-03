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
	if err := validateFilter(filter); err != nil {
		return nil, err
	}

	var err error
	var defaultArgs = dvm.NewArgs("", "", filter.Kinds[0]-1000)
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

func validateFilter(filter *nostr.Filter) error {
	if filter == nil {
		return ErrNilFilter
	}

	if filter.Search == "" {
		return ErrEmptyFieldSearch
	}

	if len(filter.Kinds) != 2 {
		return fmt.Errorf("%w :%v", ErrInvalidKindsFormat, filter.Kinds)
	}

	DVMkind, DVMerr := filter.Kinds[0], filter.Kinds[1]
	if DVMkind < 6312 || DVMkind > 6315 || DVMerr != 7000 {
		return fmt.Errorf("%w :%v", ErrInvalidKindsFormat, filter.Kinds)
	}

	return nil
}

// ParseArgs uses a streaming decoder to allow duplicate "target" keys.
// func ParseArgs(input string) (Args, error) {
// 	var args Args

// 	dec := json.NewDecoder(strings.NewReader(input))
// 	// Expect the JSON object to start with a '{'
// 	t, err := dec.Token()
// 	if err != nil {
// 		return args, err
// 	}
// 	if delim, ok := t.(json.Delim); !ok || delim != '{' {
// 		return args, fmt.Errorf("expected object start")
// 	}

// 	// Process key-value pairs manually.
// 	for dec.More() {
// 		// Read the next key.
// 		t, err := dec.Token()
// 		if err != nil {
// 			return args, err
// 		}
// 		key, ok := t.(string)
// 		if !ok {
// 			return args, fmt.Errorf("expected string key")
// 		}

// 		switch key {
// 		case "target":
// 			// Decode the target value (assuming it's a string).
// 			var target string
// 			if err := dec.Decode(&target); err != nil {
// 				return args, err
// 			}
// 			args.Targets = append(args.Targets, target)
// 		case "source":
// 			if err := dec.Decode(&args.Source); err != nil {
// 				return args, err
// 			}
// 		case "limit":
// 			if err := dec.Decode(&args.Limit); err != nil {
// 				return args, err
// 			}
// 		default:
// 			// For any key we don't care about, skip its value.
// 			if err := dec.Decode(new(interface{})); err != nil {
// 				return args, err
// 			}
// 		}
// 	}

// 	// Ensure the object ends with a '}'
// 	t, err = dec.Token()
// 	if err != nil {
// 		return args, err
// 	}
// 	if delim, ok := t.(json.Delim); !ok || delim != '}' {
// 		return args, fmt.Errorf("expected object end")
// 	}

// 	return args, nil
// }
