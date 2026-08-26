package paystack

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ChargeAuthorizationRequest is the payload to charge a reusable authorization.
type ChargeAuthorizationRequest struct {
	Amount            int64    `json:"amount"` // subunit of currency (matches InitializeRequest.Amount)
	Email             string   `json:"email"`
	AuthorizationCode string   `json:"authorization_code"`
	Reference         string   `json:"reference,omitempty"`
	Currency          string   `json:"currency,omitempty"`
	Metadata          any      `json:"metadata,omitempty"`
	Channels          []string `json:"channels,omitempty"`
	Subaccount        string   `json:"subaccount,omitempty"`
	TransactionCharge int64    `json:"transaction_charge,omitempty"`
	Bearer            string   `json:"bearer,omitempty"`
	Queue             bool     `json:"queue,omitempty"`
}

// Customer is the customer object returned on a charge/transaction.
type Customer struct {
	ID           int64  `json:"id"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Email        string `json:"email"`
	CustomerCode string `json:"customer_code"`
	Phone        string `json:"phone"`
	RiskAction   string `json:"risk_action"`
}

// ChargeAuthorizationResponse is the response from charging an authorization.
type ChargeAuthorizationResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		ID              int64         `json:"id"`
		Domain          string        `json:"domain"`
		Status          string        `json:"status"`
		Reference       string        `json:"reference"`
		Amount          int64         `json:"amount"`
		Message         string        `json:"message"`
		GatewayResponse string        `json:"gateway_response"`
		Channel         string        `json:"channel"`
		Currency        string        `json:"currency"`
		TransactionDate string        `json:"transaction_date"`
		Fees            int64         `json:"fees"`
		Authorization   Authorization `json:"authorization"`
		Customer        Customer      `json:"customer"`
		Metadata        any           `json:"metadata"`
	} `json:"data"`
}

// ChargeAuthorization charges a previously-saved (reusable) authorization code.
func (p *Paystack) ChargeAuthorization(req *ChargeAuthorizationRequest) (*ChargeAuthorizationResponse, error) {
	if err := ValidateChargeAuthorizationRequest(req); err != nil {
		return nil, err
	}
	resBody, err := p.doRequest(http.MethodPost, "/transaction/charge_authorization", req)
	if err != nil {
		return nil, err
	}
	var resp ChargeAuthorizationResponse
	if err := json.Unmarshal(resBody, &resp); err != nil {
		return nil, err
	}
	if !resp.Status {
		return nil, fmt.Errorf("paystack charge authorization failed: %s", resp.Message)
	}
	return &resp, nil
}

// ValidateChargeAuthorizationRequest checks the required fields for charging an authorization.
func ValidateChargeAuthorizationRequest(req *ChargeAuthorizationRequest) error {
	if req == nil {
		return fmt.Errorf("%w: charge authorization request is required", ErrInvalidRequest)
	}
	if req.Amount <= 0 {
		return fmt.Errorf("%w: amount must be greater than zero", ErrInvalidRequest)
	}
	if err := validateEmail(req.Email); err != nil {
		return err
	}
	if strings.TrimSpace(req.AuthorizationCode) == "" {
		return fmt.Errorf("%w: authorization code is required", ErrInvalidRequest)
	}
	if req.Currency != "" && !currencyPattern.MatchString(strings.ToUpper(req.Currency)) {
		return fmt.Errorf("%w: currency must be a 3-letter code", ErrInvalidRequest)
	}
	if req.TransactionCharge < 0 {
		return fmt.Errorf("%w: transaction charge cannot be negative", ErrInvalidRequest)
	}
	return nil
}
