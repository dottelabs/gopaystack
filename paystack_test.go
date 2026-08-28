package paystack

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestInitializeTransactionChannelsSerialization(t *testing.T) {
	withPaystackHTTPClient(t, paystackRoundTripper(func(req *http.Request) (*http.Response, error) {
		raw, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body error = %v", err)
		}
		if !strings.Contains(string(raw), `"channels":["card","bank"]`) {
			t.Errorf("request body %q missing channels", raw)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"status":true,"message":"ok","data":{"authorization_url":"https://checkout.paystack.com/x"}}`)),
			Header:     make(http.Header),
		}, nil
	}))

	req := &InitializeRequest{Amount: 5000, Email: "c@example.com", Channels: []string{"card", "bank"}}
	if _, err := NewClient("secret").InitializeTransaction(req); err != nil {
		t.Fatalf("InitializeTransaction() error = %v", err)
	}
}

func TestInitializeTransactionSubscriptionFieldsSerialization(t *testing.T) {
	withPaystackHTTPClient(t, paystackRoundTripper(func(req *http.Request) (*http.Response, error) {
		raw, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body error = %v", err)
		}
		body := string(raw)
		for _, want := range []string{`"plan":"PLN_x"`, `"invoice_limit":3`, `"split_code":"SPL_98WF13Eb3w"`} {
			if !strings.Contains(body, want) {
				t.Errorf("request body %q missing %s", body, want)
			}
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"status":true,"message":"ok","data":{"authorization_url":"https://checkout.paystack.com/x"}}`)),
			Header:     make(http.Header),
		}, nil
	}))

	req := &InitializeRequest{Email: "c@example.com", Plan: "PLN_x", InvoiceLimit: 3, SplitCode: "SPL_98WF13Eb3w"}
	if _, err := NewClient("secret").InitializeTransaction(req); err != nil {
		t.Fatalf("InitializeTransaction() error = %v", err)
	}
}

func TestInitializeTransactionOmitsEmptyChannels(t *testing.T) {
	withPaystackHTTPClient(t, paystackRoundTripper(func(req *http.Request) (*http.Response, error) {
		raw, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body error = %v", err)
		}
		if strings.Contains(string(raw), "channels") {
			t.Errorf("request body %q should omit channels when unset", raw)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"status":true,"message":"ok","data":{"authorization_url":"https://checkout.paystack.com/x"}}`)),
			Header:     make(http.Header),
		}, nil
	}))

	req := &InitializeRequest{Amount: 5000, Email: "c@example.com"}
	if _, err := NewClient("secret").InitializeTransaction(req); err != nil {
		t.Fatalf("InitializeTransaction() error = %v", err)
	}
}

func TestVerifyWebhook(t *testing.T) {
	client := NewClient("secret")
	body := []byte(`{"event":"charge.success","data":{"reference":"abc"}}`)

	mac := hmac.New(sha512.New, []byte("secret"))
	mac.Write(body)
	validSig := hex.EncodeToString(mac.Sum(nil))

	if !client.VerifyWebhook(body, validSig) {
		t.Fatal("expected valid signature to verify")
	}
	if client.VerifyWebhook(body, "bad-signature") {
		t.Fatal("expected invalid signature to fail verification")
	}
	if NewClient("").VerifyWebhook(body, validSig) {
		t.Fatal("expected empty secret to fail verification")
	}
	if client.VerifyWebhook(nil, validSig) {
		t.Fatal("expected empty body to fail verification")
	}
}

func TestValidateInitializeRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *InitializeRequest
	}{
		{name: "nil request"},
		{name: "missing amount", req: &InitializeRequest{Email: "user@example.com"}},
		{name: "missing email", req: &InitializeRequest{Amount: 1000}},
		{name: "bad email", req: &InitializeRequest{Amount: 1000, Email: "not-an-email"}},
		{name: "header injection email", req: &InitializeRequest{Amount: 1000, Email: "user@example.com\r\nBcc: attacker@example.com"}},
		{name: "bad currency", req: &InitializeRequest{Amount: 1000, Email: "user@example.com", Currency: "USDD"}},
		{name: "insecure callback", req: &InitializeRequest{Amount: 1000, Email: "user@example.com", CallbackURL: "http://example.com"}},
		{name: "negative transaction charge", req: &InitializeRequest{Amount: 1000, Email: "user@example.com", TransactionCharge: -1}},
		{name: "negative amount", req: &InitializeRequest{Amount: -1, Email: "user@example.com"}},
		{name: "negative amount with plan", req: &InitializeRequest{Amount: -1, Email: "user@example.com", Plan: "PLN_x"}},
		{name: "negative invoice limit", req: &InitializeRequest{Amount: 1000, Email: "user@example.com", InvoiceLimit: -1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateInitializeRequest(tt.req)
			if !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("ValidateInitializeRequest() error = %v, want ErrInvalidRequest", err)
			}
		})
	}

	if err := ValidateInitializeRequest(&InitializeRequest{Amount: 1000, Email: "user@example.com", CallbackURL: "https://example.com/callback", Currency: "KES"}); err != nil {
		t.Fatalf("ValidateInitializeRequest() error = %v", err)
	}

	// A plan code makes amount optional — Paystack ignores it.
	planOnly := &InitializeRequest{Email: "user@example.com", Plan: "PLN_x", InvoiceLimit: 3, SplitCode: "SPL_98WF13Eb3w"}
	if err := ValidateInitializeRequest(planOnly); err != nil {
		t.Fatalf("ValidateInitializeRequest() with plan error = %v", err)
	}
}

func TestVerifyTransactionRequiresReference(t *testing.T) {
	_, err := NewClient("secret").VerifyTransaction("")
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("VerifyTransaction() error = %v, want ErrInvalidRequest", err)
	}
}
