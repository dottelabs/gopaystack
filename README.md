# Paystack

Go client and helpers for Paystack payments.

`github.com/dottelabs/gopaystack` provides a small, typed wrapper around the Paystack API surface commonly needed by payment applications: transaction initialization, transaction verification, payment requests, refunds, splits, subaccounts, webhook verification, webhook parsing, and payment authorization detail extraction.

Application concepts such as users, invoices, coupons, database transactions, order fulfillment, and business-specific payment policy belong in the application using this module.

## Installation

```sh
go get github.com/dottelabs/gopaystack
```

```go
import "github.com/dottelabs/gopaystack" // imported as package "paystack"
```

## Features

- Initialize Paystack transactions.
- Restrict Checkout payment channels, and initialize plan subscriptions with split codes.
- Verify transactions by reference.
- Charge saved (reusable) authorizations.
- Check `Reusable` and dedupe by `Signature` before storing an authorization.
- Verify webhook signatures using Paystack's HMAC-SHA512 signature.
- Parse verified webhook payloads.
- Create and fetch Paystack payment requests.
- Create, fetch, list, and retry Paystack refunds.
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

#### Restricting payment channels

`Channels` limits which payment methods Checkout offers. Leave it empty to offer
every channel enabled on the integration. Valid values are `card`, `bank`,
`apple_pay`, `ussd`, `qr`, `mobile_money`, `bank_transfer`, `eft`, `capitec_pay`
and `payattitude`.

```go
// Card-only checkout.
resp, err := client.InitializeTransaction(&paystack.InitializeRequest{
    Amount:   250000,
    Email:    "buyer@example.com",
    Channels: []string{"card"},
})
```

Paystack does not document which channels produce a reusable authorization, and
the `channels` parameter says nothing about reusability. If you need a saved
authorization, read `reusable` on the authorization object in the verify or
webhook payload rather than inferring it from the channel. See
[Recurring Charges](https://paystack.com/docs/payments/recurring-charges/),
which documents recurring support for cards in all markets and direct debit for
Nigerian businesses. Note that direct debit is not a `channels` value — it uses
the separate Initialize Authorization endpoint.

#### Subscriptions and splits

Set `Plan` to a plan code to turn the transaction into a subscription. Paystack
ignores `Amount` when a plan is attached, so it may be left at zero.
`InvoiceLimit` caps how many times the customer is charged (zero means
unlimited), and `SplitCode` attaches a transaction split.

```go
resp, err := client.InitializeTransaction(&paystack.InitializeRequest{
    Email:        "buyer@example.com",
    Plan:         "PLN_gx2wn530m0i3w3m",
    InvoiceLimit: 12,
    SplitCode:    "SPL_98WF13Eb3w",
})
```

`ValidateInitializeRequest` checks:

- request is not nil
- amount is not negative, and is greater than zero unless `Plan` is set
- email is present and is a valid single address
- currency, when set, is a 3-letter code
- callback URL, when set, is absolute HTTPS
- transaction charge and invoice limit are not negative

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

### Saving an Authorization

Before storing an authorization for later off-session use, check
`Reusable` on the authorization object. Paystack does not guarantee
reusability by channel: card authorizations typically come back `true`, while
mobile-money authorizations typically come back `false` even though the
transaction itself succeeded. Treating a non-reusable authorization as saveable
produces a payment method that looks valid but fails at charge time.

```go
tx, err := client.VerifyTransaction(reference)
if err != nil {
    return err
}

auth := tx.Data.Authorization
if !auth.Reusable {
    // Cannot be charged off-session. Do not store it as a saved method.
    return errNotSaveable
}

// Signature is stable across charges of the same instrument, whereas
// AuthorizationCode is reissued each time. Use it to avoid saving duplicates.
if existing, ok := lookupBySignature(auth.Signature); ok {
    return refresh(existing, auth)
}
```

### Charge an Authorization

Only authorizations marked reusable can be charged; the authorization code is obtained from a previous transaction (for example, a verified webhook or `VerifyTransaction`).

```go
resp, err := client.ChargeAuthorization(&paystack.ChargeAuthorizationRequest{
    Amount:            35247,
    Email:             "customer@example.com",
    AuthorizationCode: "AUTH_uh8bcl3zbn",
})
```

`resp.Data.Status` and `resp.Data.Reference` carry the transaction outcome.

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

## Refunds

Create a refund for a transaction. `Amount` is optional and defaults to the full transaction amount when omitted.

```go
resp, err := client.CreateRefund(&paystack.RefundRequest{
    Transaction:  "TXN-8K2M9NXQ",
    Amount:       10000,
    Currency:     "KES",
    CustomerNote: "Refund for cancelled order",
})
```

`Transaction` may be a transaction reference or id.

Fetch a refund by ID:

```go
resp, err := client.GetRefund("12345")
```

`RefundResponse.Data.Transaction` is a `json.RawMessage`: Paystack returns a full transaction object on create but a bare transaction id on fetch, so unmarshal it into the appropriate type yourself.

List refunds:

```go
resp, err := client.ListRefunds(&paystack.RefundListOptions{
    Transaction: "1641",
    PerPage:     20,
})
```

A nil `*RefundListOptions` lists all refunds with Paystack's defaults. List results use `RefundListItem`, which differs from the create/fetch response: its `FullyDeducted` is an integer (`1` means fully deducted) and its timestamps are snake_case.

Retry a refund:

```go
resp, err := client.RetryRefund("1234567", &paystack.RefundAccountDetails{
    Currency:      "NGN",
    AccountNumber: "1234567890",
    BankID:        "9",
})
```

Use this for refunds in a needs-attention state.

## Errors

The module exposes these common errors:

- `ErrNotConfigured` when a nil client or empty secret key is used.
- `ErrInvalidRequest` when a required request field is missing or invalid.
- `HTTPError` for non-2xx Paystack responses, including the status code and bounded response body.

HTTP responses with status code 400 or greater are returned as errors containing the Paystack status code and response body.

## Security and Validation Notes

- Transaction references, payment request IDs, refund IDs, split IDs, and subaccount IDs are URL path-escaped before request construction.
- Empty lookup identifiers are rejected before network calls.
- Transaction initialization rejects nil requests, non-positive amounts, and missing emails.
- Transaction initialization rejects malformed emails, CRLF email input, invalid currency codes, non-HTTPS callback URLs, and negative transaction charges.
- Charge authorization rejects nil requests, non-positive amounts, missing or malformed emails (including CRLF injection), missing authorization codes, malformed currency codes, and negative transaction charges.
- Payment request creation rejects nil requests, non-positive amounts, invalid line items, and negative quantities.
- Payment request creation rejects malformed currency codes.
- Refund creation rejects nil requests, empty transaction references, negative amounts, and malformed currency codes.
- Refund retry rejects nil account details and missing currency, account number, or bank ID, and rejects malformed currency codes.
- Refund listing rejects malformed currency filters before making a network call.
- Subaccount creation rejects missing required bank/account fields and percentage charges outside `0..100`.
- Split creation rejects missing names, unsupported split types, missing currencies, empty subaccount lists, empty subaccount codes, and non-positive shares.
- Webhook verification requires a configured secret key, non-empty body, and non-empty hex signature.
- API response bodies are bounded to avoid unbounded memory use on bad upstream responses.

## Testing

```sh
go test ./...
```

Webhook tests use generated HMAC signatures and do not require network access.
