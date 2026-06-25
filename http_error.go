package paystack

import "fmt"

const maxResponseBytes = 4 << 20 // 4 MiB

// HTTPError represents a non-2xx Paystack HTTP response.
type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("paystack error (status %d): %s", e.StatusCode, e.Body)
}
