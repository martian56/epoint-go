# epoint-go

Go client for the [epoint.az](https://epoint.az) payment gateway.

Covers all 30 documented endpoints: payments, saved cards, refunds and payouts, split payments,
pre-authorisation, installments, wallets, Apple Pay and Google Pay, invoices, and B2B transfers.
No dependencies outside the standard library.

```bash
go get github.com/martian56/epoint-go
```

## Quick start

```go
package main

import (
	"context"
	"fmt"

	epoint "github.com/martian56/epoint-go"
)

func main() {
	client := epoint.New("i000000001", "your-private-key")

	payment, err := client.CreatePayment(context.Background(), 30.75, "order-1", &epoint.PaymentOptions{
		Description: "Test order",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(payment.RedirectURL())
}
```

Send the customer to `payment.RedirectURL()`. When they finish, epoint calls your result URL.
Verify it before trusting it:

```go
callback, err := client.VerifyCallback(data, signature)
if err != nil {
	http.Error(w, "invalid", http.StatusBadRequest)
	return
}
if callback.OK() {
	fulfil(callback.OrderID, callback.Transaction)
}
```

## Configuration

```go
client := epoint.New("i000000001", "your-private-key",
	epoint.WithBaseURL("https://epoint.az"),
	epoint.WithLanguage(string(epoint.LanguageAZ)),
	epoint.WithCurrency(string(epoint.CurrencyAZN)),
	epoint.WithRedirectURLs("https://shop.example/thanks", "https://shop.example/failed"),
	epoint.WithHTTPClient(customClient),
)
```

Or read from the environment with `epoint.FromEnv()`, which uses `EPOINT_PUBLIC_KEY`,
`EPOINT_PRIVATE_KEY`, `EPOINT_BASE_URL`, `EPOINT_LANGUAGE`, `EPOINT_SUCCESS_REDIRECT_URL` and
`EPOINT_FAILED_REDIRECT_URL`. Language and currency are set on the client and can be overridden per
call through the options argument. Pass `nil` for the options when you do not need any.

## Testing against the sandbox

There is a local sandbox that behaves like the real gateway, so you can build and test without a
merchant account or real money:

```go
client := epoint.New("i000000001", "sandbox_private_key_0000000001",
	epoint.WithBaseURL("http://localhost:8181"))
```

See [epoint-sandbox](https://github.com/martian56/epoint-sandbox). Switching to production means
changing the base URL and the keys, nothing else.

## Responses

Most methods return `(*Response, error)`. Known fields have accessors, anything else is read
through `Get`, `String`, or the raw map:

```go
status, err := client.GetStatus(ctx, "te0000000001")
status.Status()          // "success"
status.OK()              // true
status.String("rrn")     // bank reference number
status.Raw               // the full response map
```

A declined payment is not an error. `GetStatus` returns normally with a status of
`epoint.StatusFailed`, which you read through `OK()`. `GetInstallmentPlans` returns a slice and
`ListWallets` returns a map.

## Enums

Every value the API uses has a typed constant. They come from the sandbox's own definitions, so
they match what production sends. Each type is a `string`, so a plain string still converts.

```go
client := epoint.New(publicKey, privateKey,
	epoint.WithLanguage(string(epoint.LanguageEN)),
	epoint.WithCurrency(string(epoint.CurrencyUSD)),
)

status, err := client.GetStatus(ctx, transaction)
if err == nil && epoint.Status(status.Status()).Settled() {
	fulfil(orderID)
}
```

| Type | Values |
|---|---|
| `Status` | new, success, failed, error, returned, server_error |
| `CardStatus` | new, active, pending, rejected, expired, session_expired |
| `InvoiceStatus` | waiting_for_payment, paid, canceled |
| `B2BStatus` | PENDING, PROCESSING, SUCCESS, FAILED |
| `OperationCode` | 001 card registration, 100 payment, 200 registration with payment |
| `Language` | az, en, ru |
| `Currency` | AZN, USD, EUR, RUB |

Currency is not uniform across the API. Checkout takes all four, but split, pre-auth, refund,
reverse, payout and wallet take AZN and nothing else. `SupportedCurrencies` and `AZNOnly` hold
those two sets.

`Status.Settled()` reports whether the money moved and `CardStatus.Usable()` whether a card can
be charged.

## Methods

| Group | Methods |
|---|---|
| Checkout | `CreatePayment`, `CreatePaymentRequest`, `CreateAmexPayment`, `ChangePaymentSum` |
| Status | `GetStatus`, `GetCardStatus`, `GetBankTransfer` |
| Split | `CreateSplitPayment`, `SplitChargeSavedCard` |
| Pre-auth | `Reserve`, `Capture` |
| Saved cards | `RegisterCard`, `RegisterCardAndPay`, `ChargeSavedCard` |
| Money back | `Refund`, `Reverse` |
| Installments | `GetInstallmentPlans`, `PayByInstallment` |
| Wallets | `ListWallets`, `PayWithWallet` |
| Apple Pay, Google Pay | `CreateWidget` |
| Invoices | `CreateInvoice`, `UpdateInvoice`, `GetInvoice`, `ListInvoices`, `SendInvoiceSMS`, `SendInvoiceEmail` |
| B2B | `CreateBankTransfer`, `GetBankTransfer` |
| Health | `Heartbeat` |

## Errors

Use `errors.As` to inspect what went wrong:

| Type | Returned when |
|---|---|
| `*GatewayError` | epoint returned `status: error`. Has `Code`, `Status`, `Message`, `Payload`. |
| `*TransportError` | Network failure, or a non-JSON or `4xx`/`5xx` response. Has `StatusCode`. |
| `*SignatureError` | A callback's signature did not match, or its data would not decode |

```go
var gwErr *epoint.GatewayError
if errors.As(err, &gwErr) {
	log.Printf("declined with code %s", gwErr.Code)
}
```

## Signatures

Epoint signs with `base64(sha1_raw(private_key + data + private_key))`. The client builds and
verifies these for you. The digest is the raw 20 bytes, not the hex string, which is where most
hand-rolled integrations go wrong.

## Not affiliated with Epoint

An independent client for developers integrating with epoint.az.

MIT licensed.
