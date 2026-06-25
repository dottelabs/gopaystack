package paystack

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"testing"
)

func TestParseWebhook(t *testing.T) {
	body := []byte(`{"event":"charge.success","data":{"id":123,"reference":"TXN-1","amount":1000,"status":"success","currency":"KES","channel":"card","authorization":{"brand":"visa","last4":"4242"}}}`)
	client := NewClient("secret")
	sig := signTestWebhook(body, "secret")

	webhook, err := client.ParseWebhook(body, sig)
	if err != nil {
		t.Fatalf("ParseWebhook() error = %v", err)
	}
	if !webhook.IsChargeSuccess() {
		t.Fatal("expected charge success")
	}
	if webhook.Data.Reference != "TXN-1" {
		t.Fatalf("reference = %q", webhook.Data.Reference)
	}
	if webhook.Data.Authorization.Last4 != "4242" {
		t.Fatalf("last4 = %q", webhook.Data.Authorization.Last4)
	}
}

func TestParseWebhookRejectsBadSignature(t *testing.T) {
	client := NewClient("secret")
	if _, err := client.ParseWebhook([]byte(`{}`), "bad"); err == nil {
		t.Fatal("expected invalid signature error")
	}
}

func signTestWebhook(body []byte, secret string) string {
	hash := hmac.New(sha512.New, []byte(secret))
	hash.Write(body)
	return hex.EncodeToString(hash.Sum(nil))
}
