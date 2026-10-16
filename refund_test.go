package paystack

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestValidateRefundRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *RefundRequest
	}{
		{name: "nil"},
		{name: "empty transaction", req: &RefundRequest{}},
		{name: "whitespace transaction", req: &RefundRequest{Transaction: "   "}},
		{name: "negative amount", req: &RefundRequest{Transaction: "T685312322670591", Amount: -1}},
		{name: "invalid currency length", req: &RefundRequest{Transaction: "T685312322670591", Currency: "KESX"}},
		{name: "invalid currency chars", req: &RefundRequest{Transaction: "T685312322670591", Currency: "KE/Sh"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateRefundRequest(tt.req); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("ValidateRefundRequest() error = %v, want ErrInvalidRequest", err)
			}
		})
	}

	if err := ValidateRefundRequest(&RefundRequest{Transaction: "T685312322670591", Amount: 10000, Currency: "KES", CustomerNote: "duplicate charge"}); err != nil {
		t.Fatalf("ValidateRefundRequest(valid) error = %v", err)
	}

	if err := ValidateRefundRequest(&RefundRequest{Transaction: "T685312322670591"}); err != nil {
		t.Fatalf("ValidateRefundRequest(full refund) error = %v", err)
	}
}

// Paystack has been observed returning fully_deducted as a number (0/1) on the
// create/fetch endpoints, where this type previously required a bool and failed
// to decode a perfectly valid response.
func TestRefundNumericFullyDeducted(t *testing.T) {
	cases := map[string]struct {
		raw  string
		want bool
	}{
		"number one":  {raw: `1`, want: true},
		"number zero": {raw: `0`, want: false},
		"null":        {raw: `null`, want: false},
		"bool true":   {raw: `true`, want: true},
		"bool false":  {raw: `false`, want: false},
	}
	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			body := `{"status":true,"message":"Refund retrieved","data":{"id":1,"amount":500000,"currency":"KES","status":"processed","fully_deducted":` + tt.raw + `}}`
			withPaystackHTTPClient(t, paystackRoundTripper(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     make(http.Header),
				}, nil
			}))

			resp, err := NewClient("secret").GetRefund("1")
			if err != nil {
				t.Fatalf("GetRefund() error = %v", err)
			}
			if got := bool(resp.Data.FullyDeducted); got != tt.want {
				t.Errorf("Data.FullyDeducted = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFlexibleBoolRejectsUnsupportedJSON(t *testing.T) {
	for _, raw := range []string{`"yes"`, `{}`, `[]`} {
		var b FlexibleBool
		if err := json.Unmarshal([]byte(raw), &b); err == nil {
			t.Errorf("Unmarshal(%s) error = nil, want error", raw)
		}
	}
}

func TestRefundResponseDecoding(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		const body = `{"status":true,"message":"Refund has been queued for processing","data":{"transaction":{"id":1004723697,"reference":"T685312322670591","amount":10000,"currency":"NGN"},"integration":412829,"deducted_amount":0,"channel":null,"merchant_note":"note","customer_note":"note","status":"pending","refunded_by":"test@me.com","expected_at":"2021-12-16T09:21:17.016Z","currency":"NGN","domain":"live","amount":10000,"fully_deducted":false,"id":3018284,"createdAt":"2021-12-07T09:21:17.122Z","updatedAt":"2021-12-07T09:21:17.122Z"}}`
		withPaystackHTTPClient(t, paystackRoundTripper(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", req.Method)
			}
			if req.URL.Path != "/refund" {
				t.Fatalf("path = %s, want /refund", req.URL.Path)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}))

		resp, err := NewClient("secret").CreateRefund(&RefundRequest{Transaction: "T685312322670591"})
		if err != nil {
			t.Fatalf("CreateRefund() error = %v", err)
		}
		if resp.Data.ID != 3018284 {
			t.Errorf("Data.ID = %d, want 3018284", resp.Data.ID)
		}
		if resp.Data.Amount != 10000 {
			t.Errorf("Data.Amount = %d, want 10000", resp.Data.Amount)
		}
		if resp.Data.FullyDeducted {
			t.Errorf("Data.FullyDeducted = %v, want false", resp.Data.FullyDeducted)
		}
		if resp.Data.Status != "pending" {
			t.Errorf("Data.Status = %q, want pending", resp.Data.Status)
		}
		if resp.Data.CreatedAt != "2021-12-07T09:21:17.122Z" {
			t.Errorf("Data.CreatedAt = %q, want 2021-12-07T09:21:17.122Z", resp.Data.CreatedAt)
		}

		var tx struct {
			Reference string `json:"reference"`
		}
		if err := json.Unmarshal(resp.Data.Transaction, &tx); err != nil {
			t.Fatalf("Transaction object unmarshal error = %v", err)
		}
		if tx.Reference != "T685312322670591" {
			t.Errorf("Transaction.Reference = %q, want T685312322670591", tx.Reference)
		}
	})

	t.Run("fetch", func(t *testing.T) {
		const body = `{"status":true,"message":"Refund retrieved","data":{"integration":100982,"transaction":1641,"dispute":null,"settlement":null,"domain":"live","amount":500000,"deducted_amount":500000,"fully_deducted":true,"currency":"NGN","channel":"migs","status":"processed","refunded_by":"e@gmail.com","refunded_at":"2018-01-12T10:54:47.000Z","expected_at":"2017-10-01T21:10:59.000Z","customer_note":"xxx","merchant_note":"xxx","id":1,"createdAt":"2017-09-24T21:10:59.000Z","updatedAt":"2018-01-18T11:59:56.000Z"}}`
		withPaystackHTTPClient(t, paystackRoundTripper(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodGet {
				t.Fatalf("method = %s, want GET", req.Method)
			}
			if req.URL.Path != "/refund/1" {
				t.Fatalf("path = %s, want /refund/1", req.URL.Path)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}))

		resp, err := NewClient("secret").GetRefund("1")
		if err != nil {
			t.Fatalf("GetRefund() error = %v", err)
		}
		if resp.Data.ID != 1 {
			t.Errorf("Data.ID = %d, want 1", resp.Data.ID)
		}
		if resp.Data.Amount != 500000 {
			t.Errorf("Data.Amount = %d, want 500000", resp.Data.Amount)
		}
		if !resp.Data.FullyDeducted {
			t.Errorf("Data.FullyDeducted = %v, want true", resp.Data.FullyDeducted)
		}
		if resp.Data.Status != "processed" {
			t.Errorf("Data.Status = %q, want processed", resp.Data.Status)
		}
		if resp.Data.Channel != "migs" {
			t.Errorf("Data.Channel = %q, want migs", resp.Data.Channel)
		}

		var txID int64
		if err := json.Unmarshal(resp.Data.Transaction, &txID); err != nil {
			t.Fatalf("Transaction int unmarshal error = %v", err)
		}
		if txID != 1641 {
			t.Errorf("Transaction id = %d, want 1641", txID)
		}
	})
}

func TestValidateRefundAccountDetails(t *testing.T) {
	tests := []struct {
		name    string
		details *RefundAccountDetails
	}{
		{name: "nil"},
		{name: "missing currency", details: &RefundAccountDetails{AccountNumber: "1", BankID: "1"}},
		{name: "bad currency", details: &RefundAccountDetails{Currency: "NGNN", AccountNumber: "1234567890", BankID: "9"}},
		{name: "missing account number", details: &RefundAccountDetails{Currency: "NGN", BankID: "9"}},
		{name: "missing bank id", details: &RefundAccountDetails{Currency: "NGN", AccountNumber: "1234567890"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateRefundAccountDetails(tt.details); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("ValidateRefundAccountDetails() error = %v, want ErrInvalidRequest", err)
			}
		})
	}

	if err := ValidateRefundAccountDetails(&RefundAccountDetails{Currency: "NGN", AccountNumber: "1234567890", BankID: "9"}); err != nil {
		t.Fatalf("ValidateRefundAccountDetails(valid) error = %v", err)
	}
}

func TestListRefunds(t *testing.T) {
	const emptyBody = `{"status":true,"message":"ok","data":[]}`

	t.Run("query", func(t *testing.T) {
		withPaystackHTTPClient(t, paystackRoundTripper(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/refund" {
				t.Fatalf("path = %s, want /refund", req.URL.Path)
			}
			q := req.URL.Query()
			for key, want := range map[string]string{
				"transaction": "1641",
				"currency":    "NGN",
				"from":        "2016-09-21",
				"to":          "2016-12-31",
				"perPage":     "10",
				"page":        "2",
			} {
				if got := q.Get(key); got != want {
					t.Errorf("query %q = %q, want %q", key, got, want)
				}
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(emptyBody)),
				Header:     make(http.Header),
			}, nil
		}))

		if _, err := NewClient("secret").ListRefunds(&RefundListOptions{
			Transaction: "1641",
			Currency:    "NGN",
			From:        "2016-09-21",
			To:          "2016-12-31",
			PerPage:     10,
			Page:        2,
		}); err != nil {
			t.Fatalf("ListRefunds() error = %v", err)
		}
	})

	t.Run("no options", func(t *testing.T) {
		withPaystackHTTPClient(t, paystackRoundTripper(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/refund" {
				t.Fatalf("path = %s, want /refund", req.URL.Path)
			}
			if req.URL.RawQuery != "" {
				t.Fatalf("raw query = %q, want empty", req.URL.RawQuery)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(emptyBody)),
				Header:     make(http.Header),
			}, nil
		}))

		if _, err := NewClient("secret").ListRefunds(nil); err != nil {
			t.Fatalf("ListRefunds(nil) error = %v", err)
		}
	})

	t.Run("empty options", func(t *testing.T) {
		withPaystackHTTPClient(t, paystackRoundTripper(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/refund" {
				t.Fatalf("path = %s, want /refund", req.URL.Path)
			}
			if req.URL.RawQuery != "" {
				t.Fatalf("raw query = %q, want empty", req.URL.RawQuery)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(emptyBody)),
				Header:     make(http.Header),
			}, nil
		}))

		if _, err := NewClient("secret").ListRefunds(&RefundListOptions{}); err != nil {
			t.Fatalf("ListRefunds(empty) error = %v", err)
		}
	})

	t.Run("invalid currency", func(t *testing.T) {
		// No round-tripper installed: validation must reject before any transport call.
		if _, err := NewClient("secret").ListRefunds(&RefundListOptions{Currency: "KESX"}); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("ListRefunds(bad currency) error = %v, want ErrInvalidRequest", err)
		}
	})

	t.Run("decode", func(t *testing.T) {
		const body = `{"status":true,"message":"Refunds retrieved","data":[{"id":1,"integration":100982,"domain":"live","transaction":1641,"dispute":20,"amount":500000,"deducted_amount":500000,"currency":"NGN","channel":"migs","fully_deducted":1,"refunded_by":"c@gmail.com","refunded_at":"2018-01-12T10:54:47.000Z","expected_at":"2017-10-01T21:10:59.000Z","settlement":null,"customer_note":"xxx","merchant_note":"xxx","created_at":"2017-09-24T21:10:59.000Z","updated_at":"2018-01-18T11:59:56.000Z","status":"processed"},{"id":2,"integration":100982,"domain":"test","transaction":323896,"dispute":45,"amount":500000,"deducted_amount":null,"currency":"NGN","channel":"migs","fully_deducted":null,"refunded_by":"c@gmail.com","refunded_at":"2017-09-24T21:11:53.000Z","expected_at":"2017-10-01T21:11:53.000Z","settlement":null,"customer_note":"xxx","merchant_note":"xxx","created_at":"2017-09-24T21:11:53.000Z","updated_at":"2017-09-24T21:11:53.000Z","status":"pending"}],"meta":{"total":2,"skipped":0,"perPage":50,"page":1,"pageCount":1}}`
		withPaystackHTTPClient(t, paystackRoundTripper(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}))

		resp, err := NewClient("secret").ListRefunds(nil)
		if err != nil {
			t.Fatalf("ListRefunds() error = %v", err)
		}
		if len(resp.Data) != 2 {
			t.Fatalf("len(Data) = %d, want 2", len(resp.Data))
		}
		if resp.Data[0].ID != 1 {
			t.Errorf("Data[0].ID = %d, want 1", resp.Data[0].ID)
		}
		if resp.Data[0].Transaction != 1641 {
			t.Errorf("Data[0].Transaction = %d, want 1641", resp.Data[0].Transaction)
		}
		if resp.Data[0].Dispute != 20 {
			t.Errorf("Data[0].Dispute = %d, want 20", resp.Data[0].Dispute)
		}
		if resp.Data[0].FullyDeducted != 1 {
			t.Errorf("Data[0].FullyDeducted = %d, want 1", resp.Data[0].FullyDeducted)
		}
		if resp.Data[0].CreatedAt != "2017-09-24T21:10:59.000Z" {
			t.Errorf("Data[0].CreatedAt = %q, want 2017-09-24T21:10:59.000Z", resp.Data[0].CreatedAt)
		}
		if resp.Data[1].DeductedAmount != 0 {
			t.Errorf("Data[1].DeductedAmount = %d, want 0 (null)", resp.Data[1].DeductedAmount)
		}
		if resp.Data[1].FullyDeducted != 0 {
			t.Errorf("Data[1].FullyDeducted = %d, want 0 (null)", resp.Data[1].FullyDeducted)
		}
		if resp.Data[1].Status != "pending" {
			t.Errorf("Data[1].Status = %q, want pending", resp.Data[1].Status)
		}
		if resp.Meta.Total != 2 {
			t.Errorf("Meta.Total = %d, want 2", resp.Meta.Total)
		}
		if resp.Meta.PerPage != 50 {
			t.Errorf("Meta.PerPage = %d, want 50", resp.Meta.PerPage)
		}
		if resp.Meta.Page != 1 {
			t.Errorf("Meta.Page = %d, want 1", resp.Meta.Page)
		}
		if resp.Meta.PageCount != 1 {
			t.Errorf("Meta.PageCount = %d, want 1", resp.Meta.PageCount)
		}
	})
}

func TestRetryRefund(t *testing.T) {
	const body = `{"status":true,"message":"Refund retried and has been queued for processing","data":{"integration":123456,"transaction":3298598423,"dispute":null,"settlement":null,"id":1234567,"domain":"live","currency":"NGN","amount":20000,"status":"processing","refunded_at":null,"expected_at":"2025-10-13T16:02:18.000Z","channel":"isw_3ds","refunded_by":"p@email.com","customer_note":"note","merchant_note":"note","deducted_amount":20000,"fully_deducted":true,"bank_reference":null,"reason":"PROCESSING","customer":null,"initiated_by":"p@email.com","reversed_at":null,"session_id":null}}`
	withPaystackHTTPClient(t, paystackRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", req.Method)
		}
		if req.URL.Path != "/refund/retry_with_customer_details/1234567" {
			t.Fatalf("path = %s, want /refund/retry_with_customer_details/1234567", req.URL.Path)
		}
		raw, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body error = %v", err)
		}
		if !strings.Contains(string(raw), "refund_account_details") {
			t.Errorf("request body %q missing refund_account_details wrapper", raw)
		}
		if !strings.Contains(string(raw), "1234567890") {
			t.Errorf("request body %q missing account number", raw)
		}
		if !strings.Contains(string(raw), `"currency":"NGN"`) {
			t.Errorf("request body %q missing currency", raw)
		}
		if !strings.Contains(string(raw), `"bank_id":"9"`) {
			t.Errorf("request body %q missing bank_id", raw)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	}))

	resp, err := NewClient("secret").RetryRefund("1234567", &RefundAccountDetails{Currency: "NGN", AccountNumber: "1234567890", BankID: "9"})
	if err != nil {
		t.Fatalf("RetryRefund() error = %v", err)
	}
	if resp.Data.ID != 1234567 {
		t.Errorf("Data.ID = %d, want 1234567", resp.Data.ID)
	}
	if resp.Data.Amount != 20000 {
		t.Errorf("Data.Amount = %d, want 20000", resp.Data.Amount)
	}
	if !resp.Data.FullyDeducted {
		t.Errorf("Data.FullyDeducted = %v, want true", resp.Data.FullyDeducted)
	}
	if resp.Data.Status != "processing" {
		t.Errorf("Data.Status = %q, want processing", resp.Data.Status)
	}

	var txID int64
	if err := json.Unmarshal(resp.Data.Transaction, &txID); err != nil {
		t.Fatalf("Transaction int unmarshal error = %v", err)
	}
	if txID != 3298598423 {
		t.Errorf("Transaction id = %d, want 3298598423", txID)
	}
}
