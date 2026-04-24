// Package safe provides safe I/O helpers.
package safe

import (
	"fmt"
	"io"
)

// MaxResponseBody is the default maximum size for HTTP response bodies (10MB).
const MaxResponseBody = 10 * 1024 * 1024

// ReadAllLimited reads all bytes from r up to maxBytes.
// If maxBytes <= 0, it defaults to MaxResponseBody (10MB).
// Returns an error if the response exceeds the limit.
func ReadAllLimited(r io.Reader, maxBytes ...int64) ([]byte, error) {
	limit := int64(MaxResponseBody)
	if len(maxBytes) > 0 && maxBytes[0] > 0 {
		limit = maxBytes[0]
	}
	lr := io.LimitReader(r, limit+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response body exceeds %d bytes limit", limit)
	}
	return data, nil
}
