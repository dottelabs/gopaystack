# Paystack

Go client and helpers for Paystack payments.

`github.com/calvin-kimani/gopaystack` provides a small, typed wrapper around the Paystack API surface commonly needed by payment applications: transaction initialization, transaction verification, payment requests, splits, subaccounts, webhook verification, webhook parsing, and payment authorization detail extraction.

Application concepts such as users, invoices, coupons, database transactions, order fulfillment, and business-specific payment policy belong in the application using this module.

## Installation

```sh
go get github.com/calvin-kimani/gopaystack
```

```go
import "github.com/calvin-kimani/gopaystack" // imported as package "paystack"
```

## Features

- Initialize Paystack transactions.
- Verify transactions by reference.
- Verify webhook signatures using Paystack's HMAC-SHA512 signature.
- Parse verified webhook payloads.
- Create and fetch Paystack payment requests.
- Create and manage splits and subaccounts.
- Extract masked card, mobile-money, and bank authorization details for application metadata.
- Validate required request fields before network calls.

## Quick Start

```go
client := paystack.NewClient(os.Getenv("PAYSTACK_SECRET_KEY"))

resp, err := client.InitializeTransaction(&paystack.InitializeRequest{
    Amount:      10000, // smallest currency unit
    Email:       "customer@example.com",
    Currency:    "KES",
    Reference:   "TXN-ABC12345",
    CallbackURL: "https://example.com/payments/callback",
})
if err != nil {
    return err
}

fmt.Println(resp.Data.AuthorizationURL)
fmt.Println(resp.Data.AccessCode)
```

## Configuration

Create a client with your Paystack secret key:

```go
client := paystack.NewClient(os.Getenv("PAYSTACK_SECRET_KEY"))
```

The client uses Paystack's production API base URL:

```text
https://api.paystack.co
```

Requests use a default HTTP client with a bounded timeout. The module currently exposes a compact API focused on common payment operations rather than a fully configurable transport layer.

## Transactions

### Initialize a Transaction

```go
resp, err := client.InitializeTransaction(&paystack.InitializeRequest{
    Amount:    250000,
    Email:     "buyer@example.com",
    Currency:  "KES",
    Reference: "TXN-8K2M9NXQ",
    Metadata: map[string]string{
        "order_id": "ORD-123",
    },
})
```

`Amount` must be expressed in the smallest currency unit accepted by Paystack for the selected currency.

`ValidateInitializeRequest` checks:

- request is not nil
- amount is greater than zero
- email is present

### Verify a Transaction

```go
tx, err := client.VerifyTransaction("TXN-8K2M9NXQ")
if err != nil {
    return err
}

switch tx.Data.Status {
case paystack.TransactionStatusSuccess:
    // Mark the application payment as completed.
case paystack.TransactionStatusFailed:
    // Mark the application payment as failed.
case paystack.TransactionStatusAbandoned:
    // Mark the application payment as cancelled or abandoned.
case paystack.TransactionStatusReversed:
    // Mark the application payment as refunded/reversed.
}
```

`VerifyTransaction` rejects empty references before making a network call.

## Webhooks

Paystack webhook verification must be done against the raw request body.

```go
rawBody, err := io.ReadAll(r.Body)
if err != nil {
    return err
}

signature := r.Header.Get("x-paystack-signature")
webhook, err := client.ParseWebhook(rawBody, signature)
if err != nil {
    return err
}

if webhook.IsChargeSuccess() {
    reference := webhook.Data.Reference
    paystackID := webhook.Data.ID
    channel := webhook.Data.Channel
    details := paystack.BuildAuthorizationDetails(webhook.Data.Authorization)

    _ = reference
    _ = paystackID
    _ = channel
    _ = details
}
```

The application decides what a successful charge means. Loading an order or transaction by reference, checking idempotency, and updating local state belongs in the application.

## Authorization Details

`BuildAuthorizationDetails` extracts useful customer-facing payment details and masks mobile-money phone numbers.

```go
details := paystack.BuildAuthorizationDetails(auth)
```

The returned JSON is wrapped under a `paystack` key. It may include fields such as:

- `brand`
- `last4`
- `bin`
- `card_type`
- `bank`
- `country_code`
- `mobile_money_provider`
- `phone`

If no useful details are present, it returns nil.

## Payment Requests

```go
resp, err := client.CreatePaymentRequest(&paystack.PaymentRequest{
    Customer:    "CUS_xxx",
    Amount:      10000,
    Currency:    "KES",
    Description: "Invoice payment",
})
```

Fetch a payment request by ID or request code:

```go
resp, err := client.GetPaymentRequest("PRQ_xxx")
```

## Errors

The module exposes these common errors:

- `ErrNotConfigured` when a nil client or empty secret key is used.
- `ErrInvalidRequest` when a required request field is missing or invalid.
- `HTTPError` for non-2xx Paystack responses, including the status code and bounded response body.

HTTP responses with status code 400 or greater are returned as errors containing the Paystack status code and response body.

## Security and Validation Notes

- Transaction references, payment request IDs, split IDs, and subaccount IDs are URL path-escaped before request construction.
- Empty lookup identifiers are rejected before network calls.
- Transaction initialization rejects nil requests, non-positive amounts, and missing emails.
- Transaction initialization rejects malformed emails, CRLF email input, invalid currency codes, non-HTTPS callback URLs, and negative transaction charges.
- Payment request creation rejects nil requests, non-positive amounts, invalid line items, and negative quantities.
- Payment request creation rejects malformed currency codes.
- Subaccount creation rejects missing required bank/account fields and percentage charges outside `0..100`.
- Split creation rejects missing names, unsupported split types, missing currencies, empty subaccount lists, empty subaccount codes, and non-positive shares.
- Webhook verification requires a configured secret key, non-empty body, and non-empty hex signature.
- API response bodies are bounded to avoid unbounded memory use on bad upstream responses.

## Testing

```sh
go test ./...
```

Webhook tests use generated HMAC signatures and do not require network access.
