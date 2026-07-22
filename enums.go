package epoint

// Status is the status on a response or a callback.
type Status string

const (
	StatusNew         Status = "new"
	StatusSuccess     Status = "success"
	StatusFailed      Status = "failed"
	StatusError       Status = "error"
	StatusReturned    Status = "returned"
	StatusServerError Status = "server_error"
)

// Settled reports whether the status means the money moved.
func (s Status) Settled() bool {
	return s == StatusSuccess
}

// CardStatus is the state of a saved card.
type CardStatus string

const (
	CardStatusNew            CardStatus = "new"
	CardStatusActive         CardStatus = "active"
	CardStatusPending        CardStatus = "pending"
	CardStatusRejected       CardStatus = "rejected"
	CardStatusExpired        CardStatus = "expired"
	CardStatusSessionExpired CardStatus = "session_expired"
)

// Usable reports whether the card can be charged.
func (c CardStatus) Usable() bool {
	return c == CardStatusActive
}

// InvoiceStatus is the state of an invoice.
type InvoiceStatus string

const (
	InvoiceStatusWaiting  InvoiceStatus = "waiting_for_payment"
	InvoiceStatusPaid     InvoiceStatus = "paid"
	InvoiceStatusCanceled InvoiceStatus = "canceled"
)

// B2BStatus is the state of a business to business payment.
type B2BStatus string

const (
	B2BStatusPending    B2BStatus = "PENDING"
	B2BStatusProcessing B2BStatus = "PROCESSING"
	B2BStatusSuccess    B2BStatus = "SUCCESS"
	B2BStatusFailed     B2BStatus = "FAILED"
)

// OperationCode says what the transaction did, as it comes back on a callback.
type OperationCode string

const (
	OperationCardRegistration        OperationCode = "001"
	OperationPayment                 OperationCode = "100"
	OperationRegistrationWithPayment OperationCode = "200"
)

// Language is the language of the hosted checkout page.
type Language string

const (
	LanguageAZ Language = "az"
	LanguageEN Language = "en"
	LanguageRU Language = "ru"
)

// Currency is the payment currency.
type Currency string

const (
	CurrencyAZN Currency = "AZN"
	CurrencyUSD Currency = "USD"
	CurrencyEUR Currency = "EUR"
	CurrencyRUB Currency = "RUB"
)

// SupportedCurrencies are the ones checkout takes.
var SupportedCurrencies = []Currency{CurrencyAZN, CurrencyUSD, CurrencyEUR, CurrencyRUB}

// Split, pre-auth, refund, reverse, payout and wallet take AZN and nothing else.
var AZNOnly = []Currency{CurrencyAZN}
