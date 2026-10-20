# Changelog

## 0.4.0

- `GetStatus` returns a payment that ended in status `error` instead of a `GatewayError`. A 2xx
  get-status answer with `status: "error"` and a non-empty `transaction` is a payment state. A
  non-2xx answer or an error answer with no transaction still errors, and other methods are
  unchanged.
- `Response.OK()` is also true for the B2B `SUCCESS` and the heartbeat's `ok`.
- `CreateTokenSplitPayment` posts to `/api/1/token/split-payment` with the same arguments as
  `CreateSplitPayment`.
- `CheckoutForm` builds the signed `data` and `signature` fields and the action URL for the
  `/api/1/checkout` form post, without making a request.
- New optional parameters: `IsInstallment` on `CreatePayment`, `CreatePaymentRequest` and
  `CheckoutForm`, `BulkDescription` on `CreateBankTransfer`, and `Description` and `OtherAttr` on
  `GetInstallmentPlans`.
- Typed response accessors: `TraceID`, `Code`, `BankTransaction`, `BankResponse`,
  `OperationCode`, `RRN`, `CardName`, `CardMask`, `Amount`, `CardID`, `OrderID` and `WidgetURL`.
- `Callback` has `BankResponse`, `TraceID` and `CardID`, and reads `amount` from a number or a
  numeric string.
- `ParseB2BWebhook` parses the B2B webhook body into a `B2BWebhook`. The docs define no signature
  for it, so confirm the transfer before acting on it.

## 0.3.0

- `RegisterCard` takes `SuccessRedirectURL` and `ErrorRedirectURL`, so a card saved without a payment returns the
  customer to your own page. Without them it uses `WithRedirectURLs`, as the checkout
  methods do.

## 0.2.1

- The README documents the enums and uses them in its examples, instead of the bare strings it
  still showed.

## 0.2.0

- Enums for the values the API uses: `Status`, `CardStatus`, `InvoiceStatus`, `B2BStatus`,
  `OperationCode`, `Language` and `Currency`. Values come from the sandbox's own definitions, so
  they match what the API sends. `Currency` also records that checkout takes AZN, USD, EUR and
  RUB while split, pre-auth, refund, reverse, payout and wallet take AZN only.
- Raw strings still work everywhere they did before.

## 0.1.0

First release.

- Client covering all 30 documented endpoints: payments, saved cards, refunds and payouts,
  split payments, pre-authorisation, installments, wallets, Apple Pay and Google Pay, invoices,
  and B2B transfers.
- Request signing and callback verification.
- No dependencies outside the standard library.
- Integration tests run against the epoint-sandbox.
