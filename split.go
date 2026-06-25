package paystack

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// SplitSubaccount defines a subaccount entry within a split.
type SplitSubaccount struct {
	Subaccount string  `json:"subaccount"`
	Share      float64 `json:"share"`
}

// SplitRequest represents the payload to create a split.
type SplitRequest struct {
	Name             string            `json:"name"`
	Type             string            `json:"type"` // "percentage" or "flat"
	Currency         string            `json:"currency"`
	Subaccounts      []SplitSubaccount `json:"subaccounts"`
	BearerType       string            `json:"bearer_type,omitempty"` // "all-proportional", "all", "subaccount", "account"
	BearerSubaccount string            `json:"bearer_subaccount,omitempty"`
}

// SplitResponse represents the response from split operations.
type SplitResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		ID               int64             `json:"id"`
		Name             string            `json:"name"`
		Type             string            `json:"type"`
		Currency         string            `json:"currency"`
		Integration      int64             `json:"integration"`
		SplitCode        string            `json:"split_code"`
		Active           bool              `json:"active"`
		BearerType       string            `json:"bearer_type"`
		CreatedAt        string            `json:"created_at"`
		UpdatedAt        string            `json:"updated_at"`
		Subaccounts      []SplitSubaccount `json:"subaccounts"`
		TotalSubaccounts int               `json:"total_subaccounts"`
	} `json:"data"`
}

// CreateSplit creates a new transaction split on Paystack.
func (p *Paystack) CreateSplit(req *SplitRequest) (*SplitResponse, error) {
	if err := ValidateSplitRequest(req); err != nil {
		return nil, err
	}
	resBody, err := p.doRequest(http.MethodPost, "/split", req)
	if err != nil {
		return nil, err
	}

	var resp SplitResponse
	if err := json.Unmarshal(resBody, &resp); err != nil {
		return nil, err
	}

	if !resp.Status {
		return nil, fmt.Errorf("paystack split creation failed: %s", resp.Message)
	}

	return &resp, nil
}

// GetSplit fetches a split by ID or split code.
func (p *Paystack) GetSplit(idOrCode string) (*SplitResponse, error) {
	escaped, err := escapedPathValue("split id or code", idOrCode)
	if err != nil {
		return nil, err
	}
	resBody, err := p.doRequest(http.MethodGet, "/split/"+escaped, nil)
	if err != nil {
		return nil, err
	}

	var resp SplitResponse
	if err := json.Unmarshal(resBody, &resp); err != nil {
		return nil, err
	}

	if !resp.Status {
		return nil, fmt.Errorf("paystack get split failed: %s", resp.Message)
	}

	return &resp, nil
}

// ValidateSplitRequest checks the required fields for split creation.
func ValidateSplitRequest(req *SplitRequest) error {
	if req == nil {
		return fmt.Errorf("%w: split request is required", ErrInvalidRequest)
	}
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("%w: split name is required", ErrInvalidRequest)
	}
	if req.Type != "percentage" && req.Type != "flat" {
		return fmt.Errorf("%w: split type must be percentage or flat", ErrInvalidRequest)
	}
	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		return fmt.Errorf("%w: currency is required", ErrInvalidRequest)
	}
	if !currencyPattern.MatchString(currency) {
		return fmt.Errorf("%w: currency must be a 3-letter code", ErrInvalidRequest)
	}
	if len(req.Subaccounts) == 0 {
		return fmt.Errorf("%w: at least one subaccount is required", ErrInvalidRequest)
	}
	for i, sub := range req.Subaccounts {
		if strings.TrimSpace(sub.Subaccount) == "" {
			return fmt.Errorf("%w: subaccount %d code is required", ErrInvalidRequest, i)
		}
		if sub.Share <= 0 {
			return fmt.Errorf("%w: subaccount %d share must be greater than zero", ErrInvalidRequest, i)
		}
	}
	return nil
}
