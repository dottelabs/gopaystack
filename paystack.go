package paystack

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	netmail "net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// defaultClient is an HTTP client with a bounded timeout to prevent goroutine leaks.
var defaultClient = &http.Client{Timeout: 30 * time.Second}

const (
	BaseURL = "https://api.paystack.co"
)

var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

// Paystack is the client for Paystack API
type Paystack struct {
	SecretKey string
}

// InitializeRequest represents the payload to initialize a transaction
type InitializeRequest struct {
	Amount            int64  `json:"amount"` // in lowest denomination of Currency (e.g. cents for USD, kobo for NGN)
	Email             string `json:"email"`
	Currency          string `json:"currency,omitempty"` // NGN (default), USD, GHS, ZAR, KES
	Reference         string `json:"reference,omitempty"`
	CallbackURL       string `json:"callback_url,omitempty"`
	Subaccount        string `json:"subaccount,omitempty"`
	TransactionCharge int64  `json:"transaction_charge,omitempty"`
	Bearer            string `json:"bearer,omitempty"`
	Metadata          any    `json:"metadata,omitempty"`
}

// InitializeResponse represents the response from initializing a transaction
type InitializeResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		AuthorizationURL string `json:"authorization_url"`
		AccessCode       string `json:"access_code"`
		Reference        string `json:"reference"`
	} `json:"data"`
}

// Authorization mirrors Paystack's authorization object on a transaction
// payload. Fields vary by Channel — card fills Brand/Last4/Bin/CardType;
// mobile_money fills MobileMoney.{Provider,Phone}; bank rails fill Bank.
// All fields are optional — Paystack returns null/empty for non-applicable
// keys depending on the channel.
type Authorization struct {
	AuthorizationCode string             `json:"authorization_code,omitempty"`
	Bin               string             `json:"bin,omitempty"`
	Last4             string             `json:"last4,omitempty"`
	ExpMonth          string             `json:"exp_month,omitempty"`
	ExpYear           string             `json:"exp_year,omitempty"`
	Channel           string             `json:"channel,omitempty"`
	CardType          string             `json:"card_type,omitempty"`
	Bank              string             `json:"bank,omitempty"`
	CountryCode       string             `json:"country_code,omitempty"`
	Brand             string             `json:"brand,omitempty"`
	AccountName       string             `json:"account_name,omitempty"`
	Reusable          bool               `json:"reusable,omitempty"`
	MobileMoney       *MobileMoneyDetail `json:"mobile_money,omitempty"`
}

// MobileMoneyDetail is the mobile_money sub-object on an Authorization.
// Provider is one of: MPESA, AIRTEL, T-KASH (Kenya), MTN, ATL, VODAFONE (Ghana), etc.
type MobileMoneyDetail struct {
	Phone    string `json:"phone,omitempty"`
	Provider string `json:"provider,omitempty"`
}

// TransactionResponse represents a transaction verification response
type TransactionResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		ID              int64         `json:"id"`
		Domain          string        `json:"domain"`
		Status          string        `json:"status"`
		Reference       string        `json:"reference"`
		Amount          int64         `json:"amount"`
		GatewayResponse string        `json:"gateway_response"`
		PaidAt          time.Time     `json:"paid_at"`
		CreatedAt       time.Time     `json:"created_at"`
		Channel         string        `json:"channel"`
		Currency        string        `json:"currency"`
		Authorization   Authorization `json:"authorization"`
		Metadata        any           `json:"metadata"`
	} `json:"data"`
}

// ErrNotConfigured is returned when methods are invoked on a nil Paystack
// client (e.g. when paystack is disabled in config but the typed-nil pointer
// is still passed through an interface).
var ErrNotConfigured = fmt.Errorf("paystack: client not configured")

// ErrInvalidRequest is returned when a request is missing required fields.
var ErrInvalidRequest = errors.New("paystack: invalid request")

// NewClient creates a new Paystack client
func NewClient(secretKey string) *Paystack {
	return &Paystack{SecretKey: secretKey}
}

func (p *Paystack) doRequest(method, path string, body any) ([]byte, error) {
	if p == nil {
		return nil, ErrNotConfigured
	}
	if strings.TrimSpace(p.SecretKey) == "" {
		return nil, fmt.Errorf("%w: secret key is required", ErrNotConfigured)
	}
	url := BaseURL + path
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewBuffer(b)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+p.SecretKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := defaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(resBody) > maxResponseBytes {
		return nil, fmt.Errorf("paystack: response body exceeds %d bytes", maxResponseBytes)
	}

	if res.StatusCode >= 400 {
		return nil, &HTTPError{StatusCode: res.StatusCode, Body: string(resBody)}
	}

	return resBody, nil
}

// InitializeTransaction starts a new transaction
func (p *Paystack) InitializeTransaction(req *InitializeRequest) (*InitializeResponse, error) {
	if p == nil {
		return nil, ErrNotConfigured
	}
	if err := ValidateInitializeRequest(req); err != nil {
		return nil, err
	}
	resBody, err := p.doRequest(http.MethodPost, "/transaction/initialize", req)
	if err != nil {
		return nil, err
	}

	var resp InitializeResponse
	if err := json.Unmarshal(resBody, &resp); err != nil {
		return nil, err
	}

	if !resp.Status {
		return nil, fmt.Errorf("paystack initialization failed: %s", resp.Message)
	}

	return &resp, nil
}

// VerifyTransaction checks the status of a transaction
func (p *Paystack) VerifyTransaction(reference string) (*TransactionResponse, error) {
	escaped, err := escapedPathValue("reference", reference)
	if err != nil {
		return nil, err
	}
	resBody, err := p.doRequest(http.MethodGet, "/transaction/verify/"+escaped, nil)
	if err != nil {
		return nil, err
	}

	var resp TransactionResponse
	if err := json.Unmarshal(resBody, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// VerifyWebhook verifies that a webhook request came from Paystack
func (p *Paystack) VerifyWebhook(body []byte, signature string) bool {
	if p == nil || strings.TrimSpace(p.SecretKey) == "" || len(body) == 0 || strings.TrimSpace(signature) == "" {
		return false
	}
	hash := hmac.New(sha512.New, []byte(p.SecretKey))
	hash.Write(body)
	expected := hash.Sum(nil)
	sigBytes, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(expected, sigBytes) == 1
}

// ValidateInitializeRequest checks the required fields for transaction initialization.
func ValidateInitializeRequest(req *InitializeRequest) error {
	if req == nil {
		return fmt.Errorf("%w: initialize request is required", ErrInvalidRequest)
	}
	if req.Amount <= 0 {
		return fmt.Errorf("%w: amount must be greater than zero", ErrInvalidRequest)
	}
	if err := validateEmail(req.Email); err != nil {
		return err
	}
	if req.Currency != "" && !currencyPattern.MatchString(strings.ToUpper(req.Currency)) {
		return fmt.Errorf("%w: currency must be a 3-letter code", ErrInvalidRequest)
	}
	if req.CallbackURL != "" {
		u, err := url.Parse(strings.TrimSpace(req.CallbackURL))
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return fmt.Errorf("%w: callback URL must be absolute HTTPS", ErrInvalidRequest)
		}
	}
	if req.TransactionCharge < 0 {
		return fmt.Errorf("%w: transaction charge cannot be negative", ErrInvalidRequest)
	}
	return nil
}

// validateEmail checks that email is present, free of header-injection line
// breaks, and a syntactically valid single address (no display name).
func validateEmail(email string) error {
	if strings.TrimSpace(email) == "" {
		return fmt.Errorf("%w: email is required", ErrInvalidRequest)
	}
	if strings.ContainsAny(email, "\r\n") {
		return fmt.Errorf("%w: email contains a line break", ErrInvalidRequest)
	}
	parsed, err := netmail.ParseAddress(email)
	if err != nil || parsed.Address != email {
		return fmt.Errorf("%w: email is invalid", ErrInvalidRequest)
	}
	return nil
}

func escapedPathValue(field, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%w: %s is required", ErrInvalidRequest, field)
	}
	return url.PathEscape(value), nil
}
