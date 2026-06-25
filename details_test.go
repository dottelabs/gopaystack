package paystack

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildAuthorizationDetails_card(t *testing.T) {
	auth := Authorization{
		Brand:       "visa",
		Last4:       "4081",
		Bin:         "408408",
		CardType:    "debit",
		Bank:        "Test Bank",
		CountryCode: "KE",
	}
	out := BuildAuthorizationDetails(auth)
	if out == nil {
		t.Fatal("want JSON, got nil")
	}

	var got map[string]map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	ps, ok := got["paystack"]
	if !ok {
		t.Fatal("missing paystack namespace")
	}
	for _, key := range []string{"brand", "last4", "bin", "card_type", "bank", "country_code"} {
		if _, has := ps[key]; !has {
			t.Errorf("missing key %q in %v", key, ps)
		}
	}
	if _, has := ps["mobile_money_provider"]; has {
		t.Error("card path should not include mobile_money_provider")
	}
}

func TestBuildAuthorizationDetails_mobileMoney(t *testing.T) {
	auth := Authorization{
		Channel:     "mobile_money",
		CountryCode: "KE",
		MobileMoney: &MobileMoneyDetail{
			Provider: "MPESA",
			Phone:    "+254712345678",
		},
	}
	out := BuildAuthorizationDetails(auth)
	if out == nil {
		t.Fatal("want JSON, got nil")
	}

	var got map[string]map[string]any
	_ = json.Unmarshal(out, &got)
	ps := got["paystack"]

	if ps["mobile_money_provider"] != "MPESA" {
		t.Errorf("provider: got %v", ps["mobile_money_provider"])
	}
	phone, _ := ps["phone"].(string)
	if !strings.HasPrefix(phone, "+254712") || !strings.HasSuffix(phone, "78") || !strings.Contains(phone, "****") {
		t.Errorf("phone not masked correctly: %q", phone)
	}
	if _, has := ps["brand"]; has {
		t.Error("mobile-money path should not include brand")
	}
}

func TestBuildAuthorizationDetails_empty(t *testing.T) {
	if got := BuildAuthorizationDetails(Authorization{}); got != nil {
		t.Errorf("want nil for empty auth, got %s", got)
	}
}

func TestMaskPhone(t *testing.T) {
	cases := []struct{ in, want string }{
		{"+254712345678", "+254712****78"},
		{"0712345678", "0712****78"},
		{"123456", "****56"},
		{"12", "12"},
		{"", ""},
	}
	for _, c := range cases {
		if got := MaskPhone(c.in); got != c.want {
			t.Errorf("MaskPhone(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
