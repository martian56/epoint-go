# epoint-go

Go client for the [epoint.az](https://epoint.az) payment gateway.

Covers every documented endpoint: payments, the checkout form post, saved cards, refunds and payouts,
split payments, pre-authorisation, installments, wallets, Apple Pay and Google Pay, invoices, and B2B
transfers.
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
status.RRN()             // bank reference number
status.Amount()          // 25.5
status.Raw               // the full response map
```

The accessors are `Status`, `OK`, `Transaction`, `RedirectURL`, `Message`, `TraceID`, `Code`,
`BankTransaction`, `BankResponse`, `OperationCode`, `RRN`, `CardName`, `CardMask`, `Amount`,
`CardID`, `OrderID` and `WidgetURL`. Each returns the zero value when the field is missing.
`Amount` reads a number or a numeric string.

`OK()` is true for `success`, for the B2B `SUCCESS` and for the heartbeat's `ok`. It is false for
every other status, including `error`, `failed`, `server_error` and `returned`.

A declined payment is not an error. `GetStatus` returns normally with a status of
`epoint.StatusFailed`, which you read through `OK()`. The same goes for a payment that ended in
`error`, `server_error` or `returned`. The rule for `GetStatus` is exact: a 2xx answer with
`status: "error"` and a non-empty `transaction` string is a payment state and comes back as a
`Response`. A non-2xx answer, or an error answer with no transaction, such as an unknown
transaction, is still returned as an error. Every other method treats `status: "error"` as a
`*GatewayError`, as before. `GetInstallmentPlans` returns a slice and `ListWallets` returns a map.

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

## Saving a card without a payment

`RegisterCard` saves a card with no charge, for example when the first period is free and later
ones are charged with `ChargeSavedCard`. Keep the `card_id` it returns and send the customer to
its `RedirectURL()`.

```go
reg, err := client.RegisterCard(ctx, &epoint.RegisterCardOptions{
	Description:        "Card for renewals",
	SuccessRedirectURL: "https://shop.example/card/saved",
	ErrorRedirectURL:   "https://shop.example/card/failed",
})
if err != nil {
	return err
}
storeCard(reg.String("card_id"))
```

The customer comes back to `SuccessRedirectURL` or `ErrorRedirectURL`, or to the client's own
defaults from `WithRedirectURLs` when neither is given. A card can be charged once `GetCardStatus`
reports it `active`.

## The checkout form post

`/api/1/checkout` is a browser form post, not a JSON call. `CheckoutForm` takes the same arguments
as `CreatePayment`, makes no request, and returns the form action URL with the signed `data` and
`signature` fields. Render them as a form that submits itself, and epoint answers with a redirect to
its card page:

```go
form, err := client.CheckoutForm(30.75, "order-1", &epoint.PaymentOptions{Description: "Test order"})
if err != nil {
	return err
}
page := template.Must(template.New("pay").Parse(`<form id="pay" method="post" action="{{.URL}}">
<input type="hidden" name="data" value="{{.Data}}">
<input type="hidden" name="signature" value="{{.Signature}}">
</form>
<script>document.getElementById("pay").submit()</script>`))
page.Execute(w, form)
```

`IsInstallment` on `PaymentOptions` sends `is_installment` as 1 or 0, and is left out when nil. It
is used by `CreatePayment`, `CreatePaymentRequest` and `CheckoutForm`.

## B2B webhook

`ParseB2BWebhook` turns the B2B webhook body into a `*B2BWebhook` with `OrderID`, `Status`,
`Amount`, `PayeeIBAN`, `PayeeName` and `ErrorMessage`. Unknown fields are ignored and an unknown
status is kept as it came.

```go
hook, err := epoint.ParseB2BWebhook(body)
if err != nil {
	http.Error(w, "invalid", http.StatusBadRequest)
	return
}
transfer, err := client.GetBankTransfer(ctx, hook.OrderID)
```

The docs define no signature for this webhook, so anyone can post one. Confirm the transfer, for
example with `GetBankTransfer`, before acting on it.

## Methods

| Group | Methods |
|---|---|
| Checkout | `CreatePayment`, `CheckoutForm`, `CreatePaymentRequest`, `CreateAmexPayment`, `ChangePaymentSum` |
| Status | `GetStatus`, `GetCardStatus`, `GetBankTransfer` |
| Split | `CreateSplitPayment`, `CreateTokenSplitPayment`, `SplitChargeSavedCard` |
| Pre-auth | `Reserve`, `Capture` |
| Saved cards | `RegisterCard`, `RegisterCardAndPay`, `ChargeSavedCard` |
| Money back | `Refund`, `Reverse` |
| Installments | `GetInstallmentPlans`, `PayByInstallment` |
| Wallets | `ListWallets`, `PayWithWallet` |
| Apple Pay, Google Pay | `CreateWidget` |
| Invoices | `CreateInvoice`, `UpdateInvoice`, `GetInvoice`, `ListInvoices`, `SendInvoiceSMS`, `SendInvoiceEmail` |
| B2B | `CreateBankTransfer`, `GetBankTransfer`, `ParseB2BWebhook` |
| Health | `Heartbeat` |

`CreateTokenSplitPayment` posts to `/api/1/token/split-payment`, the Apple Pay and Google Pay
split, with the same arguments as `CreateSplitPayment`. The docs give no field for the wallet
token.

Not supported: `invoice_images[]` on invoices. The docs give no way to send it and the sandbox does
not accept it.

## Errors

Use `errors.As` to inspect what went wrong:

| Type | Returned when |
|---|---|
| `*GatewayError` | epoint returned `status: error`, except a payment state from `GetStatus`. Has `Code`, `Status`, `Message`, `Payload`. |
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
