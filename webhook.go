package paystack

import (
	"encoding/json"
	"fmt"
)

const (
	EventChargeSuccess = "charge.success"

	TransactionStatusSuccess   = "success"
	TransactionStatusFailed    = "failed"
	TransactionStatusAbandoned = "abandoned"
	TransactionStatusReversed  = "reversed"
)

// Webhook represents a Paystack webhook payload.
type Webhook struct {
	Event string `json:"event"`
	Data  struct {
		ID        int64  `json:"id"`
		Reference string `json:"reference"`
		Amount    int64  `json:"amount"`
		Status    string `json:"status"`
		Currency  string `json:"currency"`
		// Channel is the rail Paystack used, such as card, mobile_money,
		// bank_transfer, ussd, bank, qr, eft, or apple_pay.
		Channel       string        `json:"channel"`
		Authorization Authorization `json:"authorization"`
	} `json:"data"`
}

// ParseWebhook verifies the payload signature and decodes the webhook.
func (p *Paystack) ParseWebhook(body []byte, signature string) (*Webhook, error) {
	if !p.VerifyWebhook(body, signature) {
		return nil, fmt.Errorf("paystack: invalid webhook signature")
	}
	var webhook Webhook
	if err := json.Unmarshal(body, &webhook); err != nil {
		return nil, fmt.Errorf("paystack: failed to decode webhook: %w", err)
	}
	return &webhook, nil
}

// IsChargeSuccess reports whether the webhook is a successful charge event.
func (w *Webhook) IsChargeSuccess() bool {
	return w != nil && w.Event == EventChargeSuccess
}
