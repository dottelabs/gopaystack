package paystack

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"testing"
)

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
}

func TestVerifyTransactionRequiresReference(t *testing.T) {
	_, err := NewClient("secret").VerifyTransaction("")
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("VerifyTransaction() error = %v, want ErrInvalidRequest", err)
	}
}
