package paystack

import (
	"errors"
	"testing"
)

func TestEscapedPathValue(t *testing.T) {
	got, err := escapedPathValue("reference", "TXN/ABC 123")
	if err != nil {
		t.Fatalf("escapedPathValue() error = %v", err)
	}
	if got != "TXN%2FABC%20123" {
		t.Fatalf("escapedPathValue() = %q", got)
	}

	if _, err := escapedPathValue("reference", "  "); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("escapedPathValue(empty) error = %v, want ErrInvalidRequest", err)
	}
}

func TestLookupMethodsRejectEmptyIDs(t *testing.T) {
	client := NewClient("secret")
	tests := []struct {
		name string
		fn   func() error
	}{
		{name: "payment request", fn: func() error {
			_, err := client.GetPaymentRequest("")
			return err
		}},
		{name: "subaccount", fn: func() error {
			_, err := client.GetSubaccount("")
			return err
		}},
		{name: "split", fn: func() error {
			_, err := client.GetSplit("")
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.fn(); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("lookup error = %v, want ErrInvalidRequest", err)
			}
		})
	}
}

func TestValidatePaymentRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *PaymentRequest
	}{
		{name: "nil"},
		{name: "zero amount", req: &PaymentRequest{}},
		{name: "empty line item name", req: &PaymentRequest{
			Amount:    100,
			LineItems: []PaymentRequestLineItem{{Amount: 100}},
		}},
		{name: "negative line quantity", req: &PaymentRequest{
			Amount:    100,
			LineItems: []PaymentRequestLineItem{{Name: "Item", Amount: 100, Quantity: -1}},
		}},
		{name: "invalid currency", req: &PaymentRequest{Amount: 100, Currency: "KESX"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidatePaymentRequest(tt.req); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("ValidatePaymentRequest() error = %v, want ErrInvalidRequest", err)
			}
		})
	}
}

func TestValidateSubaccountRequest(t *testing.T) {
	valid := &SubaccountRequest{
		BusinessName:     "Acme",
		SettlementBank:   "test-bank",
		AccountNumber:    "1234567890",
		PercentageCharge: 10,
	}
	if err := ValidateSubaccountRequest(valid); err != nil {
		t.Fatalf("ValidateSubaccountRequest(valid) error = %v", err)
	}
	invalid := *valid
	invalid.PercentageCharge = 120
	if err := ValidateSubaccountRequest(&invalid); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("ValidateSubaccountRequest(invalid) error = %v, want ErrInvalidRequest", err)
	}
}

func TestValidateSplitRequest(t *testing.T) {
	valid := &SplitRequest{
		Name:        "Marketplace",
		Type:        "percentage",
		Currency:    "KES",
		Subaccounts: []SplitSubaccount{{Subaccount: "ACCT_123", Share: 50}},
	}
	if err := ValidateSplitRequest(valid); err != nil {
		t.Fatalf("ValidateSplitRequest(valid) error = %v", err)
	}
	invalid := *valid
	invalid.Type = "ratio"
	if err := ValidateSplitRequest(&invalid); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("ValidateSplitRequest(invalid type) error = %v, want ErrInvalidRequest", err)
	}
	invalid = *valid
	invalid.Currency = "KE/Sh"
	if err := ValidateSplitRequest(&invalid); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("ValidateSplitRequest(invalid currency) error = %v, want ErrInvalidRequest", err)
	}
}
