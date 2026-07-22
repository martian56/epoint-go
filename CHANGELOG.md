# Changelog

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
