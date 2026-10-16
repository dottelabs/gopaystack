package paystack

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// RefundRequest represents the payload to create a refund.
type RefundRequest struct {
	Transaction  string `json:"transaction"`
	Amount       int64  `json:"amount,omitempty"` // in the subunit of the currency
	Currency     string `json:"currency,omitempty"`
	CustomerNote string `json:"customer_note,omitempty"`
	MerchantNote string `json:"merchant_note,omitempty"`
}

// FlexibleBool decodes the inconsistent encodings Paystack uses for
// boolean-ish fields. Depending on the endpoint and the record, a field such as
// `fully_deducted` arrives as a JSON bool, as a 0/1 number, or as null; a plain
// bool field fails to decode the numeric form.
type FlexibleBool bool

// UnmarshalJSON accepts a JSON bool, a number (zero is false), or null.
func (b *FlexibleBool) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	switch trimmed {
	case "", "null", "false":
		*b = false
		return nil
	case "true":
		*b = true
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return fmt.Errorf("expected a boolean or a number, got %s", trimmed)
	}
	value, err := number.Float64()
	if err != nil {
		return fmt.Errorf("invalid number %s: %w", trimmed, err)
	}
	*b = value != 0
	return nil
}

// RefundResponse represents the response from refund operations.
//
// The Transaction field is polymorphic: Paystack returns a full transaction
// object when a refund is created, but a bare integer transaction ID when a
// refund is fetched. It is kept as json.RawMessage so both shapes decode
// without error; callers unmarshal it into the appropriate type.
//
// FullyDeducted is a FlexibleBool: Paystack has been observed returning it as
// both a bool and a number in this same payload shape.
type RefundResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		ID             int64           `json:"id"`
		Integration    int64           `json:"integration"`
		Domain         string          `json:"domain"`
		Transaction    json.RawMessage `json:"transaction"` // object on create, integer id on fetch
		Amount         int64           `json:"amount"`
		DeductedAmount int64           `json:"deducted_amount"`
		Currency       string          `json:"currency"`
		Channel        string          `json:"channel"`
		FullyDeducted  FlexibleBool    `json:"fully_deducted"`
		Status         string          `json:"status"`
		RefundedBy     string          `json:"refunded_by"`
		RefundedAt     string          `json:"refunded_at"`
		ExpectedAt     string          `json:"expected_at"`
		CustomerNote   string          `json:"customer_note"`
		MerchantNote   string          `json:"merchant_note"`
		CreatedAt      string          `json:"createdAt"`
		UpdatedAt      string          `json:"updatedAt"`
	} `json:"data"`
}

// CreateRefund creates a new refund on Paystack.
func (p *Paystack) CreateRefund(req *RefundRequest) (*RefundResponse, error) {
	if err := ValidateRefundRequest(req); err != nil {
		return nil, err
	}
	resBody, err := p.doRequest(http.MethodPost, "/refund", req)
	if err != nil {
		return nil, err
	}

	var resp RefundResponse
	if err := json.Unmarshal(resBody, &resp); err != nil {
		return nil, err
	}

	if !resp.Status {
		return nil, fmt.Errorf("paystack refund creation failed: %s", resp.Message)
	}

	return &resp, nil
}

// GetRefund fetches a refund by ID.
func (p *Paystack) GetRefund(id string) (*RefundResponse, error) {
	escaped, err := escapedPathValue("refund id", id)
	if err != nil {
		return nil, err
	}
	resBody, err := p.doRequest(http.MethodGet, "/refund/"+escaped, nil)
	if err != nil {
		return nil, err
	}

	var resp RefundResponse
	if err := json.Unmarshal(resBody, &resp); err != nil {
		return nil, err
	}

	if !resp.Status {
		return nil, fmt.Errorf("paystack get refund failed: %s", resp.Message)
	}

	return &resp, nil
}

// ValidateRefundRequest checks the required fields for refund creation.
func ValidateRefundRequest(req *RefundRequest) error {
	if req == nil {
		return fmt.Errorf("%w: refund request is required", ErrInvalidRequest)
	}
	if strings.TrimSpace(req.Transaction) == "" {
		return fmt.Errorf("%w: transaction reference or id is required", ErrInvalidRequest)
	}
	if req.Amount < 0 {
		return fmt.Errorf("%w: amount cannot be negative", ErrInvalidRequest)
	}
	if req.Currency != "" && !currencyPattern.MatchString(strings.ToUpper(req.Currency)) {
		return fmt.Errorf("%w: currency must be a 3-letter code", ErrInvalidRequest)
	}
	return nil
}

// RefundListItem is a refund as returned by the list endpoint. Its shape
// differs from RefundResponse.Data: fully_deducted is an integer (1 = fully
// deducted) and the timestamps are snake_case.
type RefundListItem struct {
	ID             int64  `json:"id"`
	Integration    int64  `json:"integration"`
	Domain         string `json:"domain"`
	Transaction    int64  `json:"transaction"`
	Dispute        int64  `json:"dispute"`
	Amount         int64  `json:"amount"`
	DeductedAmount int64  `json:"deducted_amount"`
	Currency       string `json:"currency"`
	Channel        string `json:"channel"`
	FullyDeducted  int    `json:"fully_deducted"`
	RefundedBy     string `json:"refunded_by"`
	RefundedAt     string `json:"refunded_at"`
	ExpectedAt     string `json:"expected_at"`
	CustomerNote   string `json:"customer_note"`
	MerchantNote   string `json:"merchant_note"`
	Status         string `json:"status"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// RefundListResponse is the response from listing refunds.
type RefundListResponse struct {
	Status  bool             `json:"status"`
	Message string           `json:"message"`
	Data    []RefundListItem `json:"data"`
	Meta    struct {
		Total     int `json:"total"`
		Skipped   int `json:"skipped"`
		PerPage   int `json:"perPage"`
		Page      int `json:"page"`
		PageCount int `json:"pageCount"`
	} `json:"meta"`
}

// RefundListOptions holds the optional query filters for ListRefunds.
// A nil *RefundListOptions lists all refunds with Paystack's defaults.
type RefundListOptions struct {
	Transaction string // filter by transaction id
	Currency    string // filter by supported currency
	From        string // start date, e.g. "2016-09-21"
	To          string // end date, e.g. "2016-09-21"
	PerPage     int    // records per page (Paystack default 50)
	Page        int    // page number (Paystack default 1)
}

// ListRefunds lists refunds, optionally filtered by the given query options.
func (p *Paystack) ListRefunds(opts *RefundListOptions) (*RefundListResponse, error) {
	path := "/refund"
	if opts != nil {
		if opts.Currency != "" && !currencyPattern.MatchString(strings.ToUpper(opts.Currency)) {
			return nil, fmt.Errorf("%w: currency must be a 3-letter code", ErrInvalidRequest)
		}
		q := url.Values{}
		if s := strings.TrimSpace(opts.Transaction); s != "" {
			q.Set("transaction", s)
		}
		if s := strings.TrimSpace(opts.Currency); s != "" {
			q.Set("currency", s)
		}
		if s := strings.TrimSpace(opts.From); s != "" {
			q.Set("from", s)
		}
		if s := strings.TrimSpace(opts.To); s != "" {
			q.Set("to", s)
		}
		if opts.PerPage > 0 {
			q.Set("perPage", strconv.Itoa(opts.PerPage))
		}
		if opts.Page > 0 {
			q.Set("page", strconv.Itoa(opts.Page))
		}
		if enc := q.Encode(); enc != "" {
			path += "?" + enc
		}
	}
	resBody, err := p.doRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var resp RefundListResponse
	if err := json.Unmarshal(resBody, &resp); err != nil {
		return nil, err
	}

	if !resp.Status {
		return nil, fmt.Errorf("paystack list refunds failed: %s", resp.Message)
	}

	return &resp, nil
}

// RefundAccountDetails is the customer bank account used to retry a refund.
type RefundAccountDetails struct {
	Currency      string `json:"currency"`
	AccountNumber string `json:"account_number"`
	BankID        string `json:"bank_id"`
}

type retryRefundBody struct {
	RefundAccountDetails *RefundAccountDetails `json:"refund_account_details"`
}

// RetryRefund retries a refund that has a needs-attention status by supplying
// the customer's bank account details. id is the previously initiated refund id.
func (p *Paystack) RetryRefund(id string, details *RefundAccountDetails) (*RefundResponse, error) {
	escaped, err := escapedPathValue("refund id", id)
	if err != nil {
		return nil, err
	}
	if err := ValidateRefundAccountDetails(details); err != nil {
		return nil, err
	}
	resBody, err := p.doRequest(http.MethodPost, "/refund/retry_with_customer_details/"+escaped, retryRefundBody{RefundAccountDetails: details})
	if err != nil {
		return nil, err
	}

	var resp RefundResponse
	if err := json.Unmarshal(resBody, &resp); err != nil {
		return nil, err
	}

	if !resp.Status {
		return nil, fmt.Errorf("paystack retry refund failed: %s", resp.Message)
	}

	return &resp, nil
}

// ValidateRefundAccountDetails checks the required fields for retrying a refund.
func ValidateRefundAccountDetails(details *RefundAccountDetails) error {
	if details == nil {
		return fmt.Errorf("%w: refund account details are required", ErrInvalidRequest)
	}
	currency := strings.ToUpper(strings.TrimSpace(details.Currency))
	if currency == "" {
		return fmt.Errorf("%w: currency is required", ErrInvalidRequest)
	}
	if !currencyPattern.MatchString(currency) {
		return fmt.Errorf("%w: currency must be a 3-letter code", ErrInvalidRequest)
	}
	if strings.TrimSpace(details.AccountNumber) == "" {
		return fmt.Errorf("%w: account number is required", ErrInvalidRequest)
	}
	if strings.TrimSpace(details.BankID) == "" {
		return fmt.Errorf("%w: bank id is required", ErrInvalidRequest)
	}
	return nil
}
