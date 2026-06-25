package paystack

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// PaymentRequestLineItem represents a line item on an invoice.
type PaymentRequestLineItem struct {
	Name     string  `json:"name"`
	Amount   float64 `json:"amount"`
	Quantity int     `json:"quantity,omitempty"`
}

// PaymentRequest represents the payload to create a payment request (invoice).
type PaymentRequest struct {
	Customer         string                   `json:"customer,omitempty"`
	Amount           float64                  `json:"amount"`
	Currency         string                   `json:"currency,omitempty"`
	Description      string                   `json:"description,omitempty"`
	LineItems        []PaymentRequestLineItem `json:"line_items,omitempty"`
	Tax              []any                    `json:"tax,omitempty"`
	SendNotification *bool                    `json:"send_notification,omitempty"`
	DueDate          string                   `json:"due_date,omitempty"`
	Metadata         any                      `json:"metadata,omitempty"`
	SplitCode        string                   `json:"split_code,omitempty"`
}

// PaymentRequestResponse represents the response from payment request operations.
type PaymentRequestResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		ID            int64   `json:"id"`
		RequestCode   string  `json:"request_code"`
		Status        string  `json:"status"`
		Amount        float64 `json:"amount"`
		Currency      string  `json:"currency"`
		Description   string  `json:"description"`
		Integration   int64   `json:"integration"`
		HasInvoice    bool    `json:"has_invoice"`
		InvoiceNumber int64   `json:"invoice_number"`
		CreatedAt     string  `json:"created_at"`
		UpdatedAt     string  `json:"updated_at"`
		Metadata      any     `json:"metadata"`
		Paid          bool    `json:"paid"`
	} `json:"data"`
}

// CreatePaymentRequest creates a new payment request (invoice) on Paystack.
func (p *Paystack) CreatePaymentRequest(req *PaymentRequest) (*PaymentRequestResponse, error) {
	if err := ValidatePaymentRequest(req); err != nil {
		return nil, err
	}
	resBody, err := p.doRequest(http.MethodPost, "/paymentrequest", req)
	if err != nil {
		return nil, err
	}

	var resp PaymentRequestResponse
	if err := json.Unmarshal(resBody, &resp); err != nil {
		return nil, err
	}

	if !resp.Status {
		return nil, fmt.Errorf("paystack payment request creation failed: %s", resp.Message)
	}

	return &resp, nil
}

// GetPaymentRequest fetches a payment request by ID or request code.
func (p *Paystack) GetPaymentRequest(idOrCode string) (*PaymentRequestResponse, error) {
	escaped, err := escapedPathValue("payment request id or code", idOrCode)
	if err != nil {
		return nil, err
	}
	resBody, err := p.doRequest(http.MethodGet, "/paymentrequest/"+escaped, nil)
	if err != nil {
		return nil, err
	}

	var resp PaymentRequestResponse
	if err := json.Unmarshal(resBody, &resp); err != nil {
		return nil, err
	}

	if !resp.Status {
		return nil, fmt.Errorf("paystack get payment request failed: %s", resp.Message)
	}

	return &resp, nil
}

// ValidatePaymentRequest checks the required fields for payment request creation.
func ValidatePaymentRequest(req *PaymentRequest) error {
	if req == nil {
		return fmt.Errorf("%w: payment request is required", ErrInvalidRequest)
	}
	if req.Amount <= 0 {
		return fmt.Errorf("%w: amount must be greater than zero", ErrInvalidRequest)
	}
	if req.Currency != "" && !currencyPattern.MatchString(strings.ToUpper(req.Currency)) {
		return fmt.Errorf("%w: currency must be a 3-letter code", ErrInvalidRequest)
	}
	for i, item := range req.LineItems {
		if strings.TrimSpace(item.Name) == "" {
			return fmt.Errorf("%w: line item %d name is required", ErrInvalidRequest, i)
		}
		if item.Amount <= 0 {
			return fmt.Errorf("%w: line item %d amount must be greater than zero", ErrInvalidRequest, i)
		}
		if item.Quantity < 0 {
			return fmt.Errorf("%w: line item %d quantity cannot be negative", ErrInvalidRequest, i)
		}
	}
	return nil
}
