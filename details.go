// Helpers for surfacing per-transaction payment details (card brand, mobile
// money provider, masked phone, etc.) from a Paystack authorization payload.
package paystack

import "encoding/json"

// BuildAuthorizationDetails extracts the user-relevant fields from a Paystack
// Authorization object into a JSON object wrapped under a "paystack" key,
// suitable for merging into a transactions/invoices metadata JSONB column
// without colliding with caller-supplied keys.
//
// Returns nil when nothing useful is present, so callers can leave the
// metadata column untouched. The top-level channel string lives in its own
// dedicated column; this blob carries the finer-grained rail details
// Paystack reports (card brand + last4, mobile money provider + phone, etc.).
func BuildAuthorizationDetails(auth Authorization) json.RawMessage {
	details := map[string]any{}
	if auth.Brand != "" {
		details["brand"] = auth.Brand
	}
	if auth.Last4 != "" {
		details["last4"] = auth.Last4
	}
	if auth.Bin != "" {
		details["bin"] = auth.Bin
	}
	if auth.CardType != "" {
		// "debit" / "credit" / occasionally a brand-qualified variant like
		// "visa debit". Surface verbatim — Paystack is authoritative.
		details["card_type"] = auth.CardType
	}
	if auth.Bank != "" {
		details["bank"] = auth.Bank
	}
	if auth.CountryCode != "" {
		details["country_code"] = auth.CountryCode
	}
	if auth.MobileMoney != nil {
		if auth.MobileMoney.Provider != "" {
			// MPESA / AIRTEL / T-KASH (Kenya), MTN / VODAFONE / ATL (Ghana), etc.
			details["mobile_money_provider"] = auth.MobileMoney.Provider
		}
		if auth.MobileMoney.Phone != "" {
			details["phone"] = MaskPhone(auth.MobileMoney.Phone)
		}
	}

	if len(details) == 0 {
		return nil
	}
	wrapped := map[string]any{"paystack": details}
	b, err := json.Marshal(wrapped)
	if err != nil {
		return nil
	}
	return b
}

// MaskPhone preserves the country/operator prefix and the last two digits
// while replacing four middle digits with stars, e.g. "+254712345678" →
// "+254712****78". Numbers shorter than seven characters mask everything
// except the last two digits.
func MaskPhone(p string) string {
	r := []rune(p)
	if len(r) <= 6 {
		if len(r) <= 2 {
			return p
		}
		for i := 0; i < len(r)-2; i++ {
			r[i] = '*'
		}
		return string(r)
	}
	for i := len(r) - 6; i < len(r)-2; i++ {
		r[i] = '*'
	}
	return string(r)
}
