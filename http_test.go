package paystack

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type paystackRoundTripper func(req *http.Request) (*http.Response, error)

func (f paystackRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func withPaystackHTTPClient(t *testing.T, rt http.RoundTripper) {
	t.Helper()
	old := defaultClient
	defaultClient = &http.Client{Transport: rt}
	t.Cleanup(func() { defaultClient = old })
}

func TestDoRequestReturnsTypedHTTPError(t *testing.T) {
	withPaystackHTTPClient(t, paystackRoundTripper(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(strings.NewReader(`{"message":"bad"}`)),
			Header:     make(http.Header),
		}, nil
	}))

	_, err := NewClient("secret").doRequest(http.MethodGet, "/test", nil)
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("doRequest() error = %v, want HTTPError", err)
	}
	if httpErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("HTTPError status = %d", httpErr.StatusCode)
	}
}

func TestDoRequestRejectsOversizedResponse(t *testing.T) {
	withPaystackHTTPClient(t, paystackRoundTripper(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(io.LimitReader(infiniteReader{}, maxResponseBytes+1)),
			Header:     make(http.Header),
		}, nil
	}))

	_, err := NewClient("secret").doRequest(http.MethodGet, "/test", nil)
	if err == nil || !strings.Contains(err.Error(), "response body exceeds") {
		t.Fatalf("doRequest() error = %v, want oversized response error", err)
	}
}

type infiniteReader struct{}

func (infiniteReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	return len(p), nil
}
