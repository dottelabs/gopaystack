package paystack

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestValidateChargeAuthorizationRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *ChargeAuthorizationRequest
	}{
		{name: "nil"},
		{name: "zero amount", req: &ChargeAuthorizationRequest{Email: "c@example.com", AuthorizationCode: "AUTH_x"}},
		{name: "missing email", req: &ChargeAuthorizationRequest{Amount: 1000, AuthorizationCode: "AUTH_x"}},
		{name: "bad email", req: &ChargeAuthorizationRequest{Amount: 1000, Email: "not-an-email", AuthorizationCode: "AUTH_x"}},
		{name: "crlf email", req: &ChargeAuthorizationRequest{Email: "c@example.com\r\nBcc: e@evil.com", Amount: 1000, AuthorizationCode: "AUTH_x"}},
		{name: "missing authorization code", req: &ChargeAuthorizationRequest{Amount: 1000, Email: "c@example.com"}},
		{name: "bad currency", req: &ChargeAuthorizationRequest{Amount: 1000, Email: "c@example.com", AuthorizationCode: "AUTH_x", Currency: "NGNN"}},
		{name: "negative transaction charge", req: &ChargeAuthorizationRequest{Amount: 1000, Email: "c@example.com", AuthorizationCode: "AUTH_x", TransactionCharge: -1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateChargeAuthorizationRequest(tt.req); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("ValidateChargeAuthorizationRequest() error = %v, want ErrInvalidRequest", err)
			}
		})
	}

	if err := ValidateChargeAuthorizationRequest(&ChargeAuthorizationRequest{Amount: 35247, Email: "c@example.com", AuthorizationCode: "AUTH_uh8bcl3zbn", Currency: "NGN"}); err != nil {
		t.Fatalf("ValidateChargeAuthorizationRequest(valid) error = %v", err)
	}
}

func TestChargeAuthorization(t *testing.T) {
	const body = `{"status":true,"message":"Charge attempted","data":{"amount":35247,"currency":"NGN","transaction_date":"2024-08-22T10:53:49.000Z","status":"success","reference":"0m7frfnr47ezyxl","domain":"test","metadata":"","gateway_response":"Approved","message":null,"channel":"card","ip_address":null,"log":null,"fees":10247,"authorization":{"authorization_code":"AUTH_uh8bcl3zbn","bin":"408408","last4":"4081","exp_month":"12","exp_year":"2030","channel":"card","card_type":"visa","bank":"TEST BANK","country_code":"NG","brand":"visa","reusable":true,"signature":"SIG_yEXu7dLBeqG0kU7g95Ke","account_name":null},"customer":{"id":181873746,"first_name":null,"last_name":null,"email":"customer@example.com","customer_code":"CUS_1rkzaqsv4rrhqo6","phone":null,"metadata":null,"risk_action":"default","international_format_phone":null},"plan":null,"id":4099490251}}`
	withPaystackHTTPClient(t, paystackRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", req.Method)
		}
		if req.URL.Path != "/transaction/charge_authorization" {
			t.Fatalf("path = %s, want /transaction/charge_authorization", req.URL.Path)
		}
		raw, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body error = %v", err)
		}
		for _, want := range []string{`"authorization_code"`, "AUTH_uh8bcl3zbn", `"email"`, `"amount"`} {
			if !strings.Contains(string(raw), want) {
				t.Errorf("request body %q missing %s", raw, want)
			}
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	}))

	resp, err := NewClient("secret").ChargeAuthorization(&ChargeAuthorizationRequest{Amount: 35247, Email: "customer@example.com", AuthorizationCode: "AUTH_uh8bcl3zbn"})
	if err != nil {
		t.Fatalf("ChargeAuthorization() error = %v", err)
	}
	if resp.Data.ID != 4099490251 {
		t.Errorf("Data.ID = %d, want 4099490251", resp.Data.ID)
	}
	if resp.Data.Status != "success" {
		t.Errorf("Data.Status = %q, want success", resp.Data.Status)
	}
	if resp.Data.Reference != "0m7frfnr47ezyxl" {
		t.Errorf("Data.Reference = %q, want 0m7frfnr47ezyxl", resp.Data.Reference)
	}
	if resp.Data.Amount != 35247 {
		t.Errorf("Data.Amount = %d, want 35247", resp.Data.Amount)
	}
	if resp.Data.Fees != 10247 {
		t.Errorf("Data.Fees = %d, want 10247", resp.Data.Fees)
	}
	if resp.Data.Currency != "NGN" {
		t.Errorf("Data.Currency = %q, want NGN", resp.Data.Currency)
	}
	if resp.Data.Channel != "card" {
		t.Errorf("Data.Channel = %q, want card", resp.Data.Channel)
	}
	if resp.Data.GatewayResponse != "Approved" {
		t.Errorf("Data.GatewayResponse = %q, want Approved", resp.Data.GatewayResponse)
	}
	if resp.Data.TransactionDate != "2024-08-22T10:53:49.000Z" {
		t.Errorf("Data.TransactionDate = %q, want 2024-08-22T10:53:49.000Z", resp.Data.TransactionDate)
	}
	if resp.Data.Authorization.AuthorizationCode != "AUTH_uh8bcl3zbn" {
		t.Errorf("Authorization.AuthorizationCode = %q, want AUTH_uh8bcl3zbn", resp.Data.Authorization.AuthorizationCode)
	}
	if !resp.Data.Authorization.Reusable {
		t.Errorf("Authorization.Reusable = %v, want true", resp.Data.Authorization.Reusable)
	}
	if resp.Data.Authorization.Last4 != "4081" {
		t.Errorf("Authorization.Last4 = %q, want 4081", resp.Data.Authorization.Last4)
	}
	if resp.Data.Customer.ID != 181873746 {
		t.Errorf("Customer.ID = %d, want 181873746", resp.Data.Customer.ID)
	}
	if resp.Data.Customer.CustomerCode != "CUS_1rkzaqsv4rrhqo6" {
		t.Errorf("Customer.CustomerCode = %q, want CUS_1rkzaqsv4rrhqo6", resp.Data.Customer.CustomerCode)
	}
	if resp.Data.Customer.RiskAction != "default" {
		t.Errorf("Customer.RiskAction = %q, want default", resp.Data.Customer.RiskAction)
	}
}

func TestChargeAuthorizationStatusFalse(t *testing.T) {
	withPaystackHTTPClient(t, paystackRoundTripper(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"status":false,"message":"Transaction not found","data":null}`)),
			Header:     make(http.Header),
		}, nil
	}))

	_, err := NewClient("secret").ChargeAuthorization(&ChargeAuthorizationRequest{Amount: 1000, Email: "c@example.com", AuthorizationCode: "AUTH_x"})
	if err == nil {
		t.Fatal("ChargeAuthorization() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "paystack charge authorization failed") {
		t.Errorf("error = %v, want to contain %q", err, "paystack charge authorization failed")
	}
	if !strings.Contains(err.Error(), "Transaction not found") {
		t.Errorf("error = %v, want to contain %q", err, "Transaction not found")
	}
}

func TestChargeAuthorizationHTTPError(t *testing.T) {
	withPaystackHTTPClient(t, paystackRoundTripper(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(strings.NewReader(`{"status":false,"message":"Invalid authorization code"}`)),
			Header:     make(http.Header),
		}, nil
	}))

	_, err := NewClient("secret").ChargeAuthorization(&ChargeAuthorizationRequest{Amount: 1000, Email: "c@example.com", AuthorizationCode: "AUTH_x"})
	var he *HTTPError
	if !errors.As(err, &he) {
		t.Fatalf("ChargeAuthorization() error = %v, want *HTTPError", err)
	}
	if he.StatusCode != http.StatusBadRequest {
		t.Errorf("HTTPError.StatusCode = %d, want %d", he.StatusCode, http.StatusBadRequest)
	}
}

func TestChargeAuthorizationDeclined(t *testing.T) {
	// A declined charge is still a successful API call: err is nil and the
	// caller must inspect resp.Data.Status (e.g. "failed") to detect the decline.
	withPaystackHTTPClient(t, paystackRoundTripper(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"status":true,"message":"Charge attempted","data":{"amount":35247,"currency":"NGN","status":"failed","reference":"declined_ref","gateway_response":"Declined","authorization":{"authorization_code":"AUTH_x","reusable":false},"customer":{"customer_code":"CUS_x"},"id":123}}`)),
			Header:     make(http.Header),
		}, nil
	}))

	resp, err := NewClient("secret").ChargeAuthorization(&ChargeAuthorizationRequest{Amount: 1000, Email: "c@example.com", AuthorizationCode: "AUTH_x"})
	if err != nil {
		t.Fatalf("ChargeAuthorization() error = %v, want nil for a declined charge", err)
	}
	if resp.Data.Status != "failed" {
		t.Errorf("Data.Status = %q, want failed", resp.Data.Status)
	}
	if resp.Data.GatewayResponse != "Declined" {
		t.Errorf("Data.GatewayResponse = %q, want Declined", resp.Data.GatewayResponse)
	}
	if resp.Data.Reference != "declined_ref" {
		t.Errorf("Data.Reference = %q, want declined_ref", resp.Data.Reference)
	}
}

func TestChargeAuthorizationQueueSerialization(t *testing.T) {
	withPaystackHTTPClient(t, paystackRoundTripper(func(req *http.Request) (*http.Response, error) {
		raw, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body error = %v", err)
		}
		if !strings.Contains(string(raw), `"queue":true`) {
			t.Errorf("request body %q missing queue flag", raw)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"status":true,"message":"ok","data":{"status":"success"}}`)),
			Header:     make(http.Header),
		}, nil
	}))

	if _, err := NewClient("secret").ChargeAuthorization(&ChargeAuthorizationRequest{Amount: 1000, Email: "c@example.com", AuthorizationCode: "AUTH_x", Queue: true}); err != nil {
		t.Fatalf("ChargeAuthorization() error = %v", err)
	}
}
