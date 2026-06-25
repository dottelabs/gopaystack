package paystack

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// SubaccountRequest represents the payload to create a subaccount
type SubaccountRequest struct {
	BusinessName        string  `json:"business_name"`
	SettlementBank      string  `json:"settlement_bank"`
	AccountNumber       string  `json:"account_number"`
	PercentageCharge    float64 `json:"percentage_charge,omitempty"`
	Description         string  `json:"description,omitempty"`
	PrimaryContactEmail string  `json:"primary_contact_email,omitempty"`
	PrimaryContactName  string  `json:"primary_contact_name,omitempty"`
	PrimaryContactPhone string  `json:"primary_contact_phone,omitempty"`
	Metadata            any     `json:"metadata,omitempty"`
}

// SubaccountResponse represents the response from subaccount operations
type SubaccountResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		ID                  int64   `json:"id"`
		SubaccountCode      string  `json:"subaccount_code"`
		BusinessName        string  `json:"business_name"`
		Description         string  `json:"description"`
		PrimaryContactName  string  `json:"primary_contact_name"`
		PrimaryContactEmail string  `json:"primary_contact_email"`
		PrimaryContactPhone string  `json:"primary_contact_phone"`
		PercentageCharge    float64 `json:"percentage_charge"`
		SettlementBank      string  `json:"settlement_bank"`
		AccountNumber       string  `json:"account_number"`
		Active              bool    `json:"active"`
		Metadata            any     `json:"metadata"`
		CreatedAt           string  `json:"created_at"`
		UpdatedAt           string  `json:"updated_at"`
	} `json:"data"`
}

// CreateSubaccount creates a new subaccount on Paystack.
func (p *Paystack) CreateSubaccount(req *SubaccountRequest) (*SubaccountResponse, error) {
	if err := ValidateSubaccountRequest(req); err != nil {
		return nil, err
	}
	resBody, err := p.doRequest(http.MethodPost, "/subaccount", req)
	if err != nil {
		return nil, err
	}

	var resp SubaccountResponse
	if err := json.Unmarshal(resBody, &resp); err != nil {
		return nil, err
	}

	if !resp.Status {
		return nil, fmt.Errorf("paystack subaccount creation failed: %s", resp.Message)
	}

	return &resp, nil
}

// GetSubaccount fetches a subaccount by ID or subaccount code.
func (p *Paystack) GetSubaccount(idOrCode string) (*SubaccountResponse, error) {
	escaped, err := escapedPathValue("subaccount id or code", idOrCode)
	if err != nil {
		return nil, err
	}
	resBody, err := p.doRequest(http.MethodGet, "/subaccount/"+escaped, nil)
	if err != nil {
		return nil, err
	}

	var resp SubaccountResponse
	if err := json.Unmarshal(resBody, &resp); err != nil {
		return nil, err
	}

	if !resp.Status {
		return nil, fmt.Errorf("paystack get subaccount failed: %s", resp.Message)
	}

	return &resp, nil
}

// ValidateSubaccountRequest checks the required fields for subaccount creation.
func ValidateSubaccountRequest(req *SubaccountRequest) error {
	if req == nil {
		return fmt.Errorf("%w: subaccount request is required", ErrInvalidRequest)
	}
	if strings.TrimSpace(req.BusinessName) == "" {
		return fmt.Errorf("%w: business name is required", ErrInvalidRequest)
	}
	if strings.TrimSpace(req.SettlementBank) == "" {
		return fmt.Errorf("%w: settlement bank is required", ErrInvalidRequest)
	}
	if strings.TrimSpace(req.AccountNumber) == "" {
		return fmt.Errorf("%w: account number is required", ErrInvalidRequest)
	}
	if req.PercentageCharge < 0 || req.PercentageCharge > 100 {
		return fmt.Errorf("%w: percentage charge must be between 0 and 100", ErrInvalidRequest)
	}
	return nil
}
