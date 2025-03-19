// The package req parses normal relay REQ into our dvm arguments. Specifically,
// the field filter.Search MUST be escaped JSON, and used to pass all parameters (e.g. "source", "target"...)
package req

import (
	"encoding/json"
	"errors"
	"fmt"

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
// This function should always be called after [ValidateFilter].
func Parse(filter *nostr.Filter) (*dvm.Request, error) {
	if len(filter.Search) < 1 {
		return nil, ErrEmptyFieldSearch
	}

	record := dvm.Record{Kind: filter.Kinds[0] - 1000, CreatedAt: nostr.Now()}
	request := dvm.NewRequest(record)

	if err := json.Unmarshal([]byte(filter.Search), &request); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnmarshalling, err)
	}

	return &request, nil
}

// ValidateFilter checks if the kinds of the filter match the valid format kinds:{<dvm_response_kind>, 7000}.
func ValidateFilter(filter *nostr.Filter) error {
	if len(filter.Kinds) != 2 {
		return fmt.Errorf("%w :%v", ErrInvalidKindsFormat, filter.Kinds)
	}

	DVMkind, DVMerr := filter.Kinds[0], filter.Kinds[1]
	if DVMkind < 6312 || DVMkind > 6315 || DVMerr != 7000 {
		return fmt.Errorf("%w :%v", ErrInvalidKindsFormat, filter.Kinds)
	}

	return nil
}
