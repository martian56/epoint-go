package epoint

import (
	"context"
	"net/http"
)

type PaymentOptions struct {
	Description        string
	Language           string
	Currency           string
	SuccessRedirectURL string
	ErrorRedirectURL   string
	OtherAttr          any
	IsInstallment      *bool
}

type SignedForm struct {
	URL       string
	Data      string
	Signature string
}

type ChargeOptions struct {
	Description string
	Language    string
	Currency    string
}

type ReverseOptions struct {
	Amount   *float64
	Language string
	Currency string
}

type RegisterCardOptions struct {
	Description        string
	Language           string
	ForPayouts         bool
	SuccessRedirectURL string
	ErrorRedirectURL   string
}

type InstallmentQueryOptions struct {
	Amount      *float64
	Language    string
	Currency    string
	Description string
	OtherAttr   any
}

type InvoiceOptions struct {
	Name             string
	Description      string
	Phone            string
	Email            string
	TIN              string
	ContractNumber   string
	MerchantOrderID  string
	Hidden           bool
	SaveAsTemplate   bool
	AllowInstallment bool
}

type BankTransfer struct {
	OrderID         string
	Amount          float64
	Description     string
	IBAN            string
	Name            string
	TIN             string
	BankCode        string
	BankName        string
	Email           string
	Address         string
	AdditionalInfo  string
	WebhookURL      string
	BulkDescription string
}

func (c *Client) checkout(amount float64, orderID string, opts *PaymentOptions) map[string]any {
	if opts == nil {
		opts = &PaymentOptions{}
	}
	return map[string]any{
		"amount":               money(amount),
		"currency":             orDefault(opts.Currency, c.currency),
		"language":             orDefault(opts.Language, c.language),
		"order_id":             orderID,
		"description":          opts.Description,
		"other_attr":           opts.OtherAttr,
		"success_redirect_url": orDefault(opts.SuccessRedirectURL, c.successRedirectURL),
		"error_redirect_url":   orDefault(opts.ErrorRedirectURL, c.errorRedirectURL),
	}
}

func (c *Client) installmentCheckout(amount float64, orderID string, opts *PaymentOptions) map[string]any {
	payload := c.checkout(amount, orderID, opts)
	if opts != nil && opts.IsInstallment != nil {
		payload["is_installment"] = boolInt(*opts.IsInstallment)
	}
	return payload
}

func orDefault(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func (c *Client) CreatePayment(ctx context.Context, amount float64, orderID string, opts *PaymentOptions) (*Response, error) {
	return c.post(ctx, "/api/1/request", c.installmentCheckout(amount, orderID, opts))
}

func (c *Client) CheckoutForm(amount float64, orderID string, opts *PaymentOptions) (*SignedForm, error) {
	return c.signedForm("/api/1/checkout", c.installmentCheckout(amount, orderID, opts))
}

func (c *Client) CreatePaymentRequest(ctx context.Context, amount float64, orderID string, opts *PaymentOptions) (*Response, error) {
	return c.post(ctx, "/api/1/payment-request", c.installmentCheckout(amount, orderID, opts))
}

func (c *Client) CreateAmexPayment(ctx context.Context, amount float64, orderID string, opts *PaymentOptions) (*Response, error) {
	return c.post(ctx, "/api/1/amex-request", c.checkout(amount, orderID, opts))
}

func (c *Client) ChangePaymentSum(ctx context.Context, amount float64, orderID string, opts *PaymentOptions) (*Response, error) {
	return c.post(ctx, "/api/1/payment-change-sum", c.checkout(amount, orderID, opts))
}

func (c *Client) Reserve(ctx context.Context, amount float64, orderID string, opts *PaymentOptions) (*Response, error) {
	return c.post(ctx, "/api/1/pre-auth-request", c.checkout(amount, orderID, opts))
}

func (c *Client) Capture(ctx context.Context, transaction string, amount float64) (*Response, error) {
	return c.post(ctx, "/api/1/pre-auth-complete", map[string]any{
		"transaction": transaction,
		"amount":      money(amount),
	})
}

func (c *Client) GetStatus(ctx context.Context, transaction string) (*Response, error) {
	status, parsed, err := c.sendForm(ctx, http.MethodPost, "/api/1/get-status", map[string]any{"transaction": transaction})
	if err != nil {
		return nil, err
	}
	return parsePaymentState(status, parsed)
}

func (c *Client) GetCardStatus(ctx context.Context, cardID string) (*Response, error) {
	return c.post(ctx, "/api/1/get-status-card", map[string]any{"card_id": cardID})
}

func (c *Client) CreateSplitPayment(ctx context.Context, amount float64, orderID, splitAccount string, splitAmount float64, opts *PaymentOptions) (*Response, error) {
	payload := c.checkout(amount, orderID, opts)
	payload["split_user"] = splitAccount
	payload["split_amount"] = money(splitAmount)
	return c.post(ctx, "/api/1/split-request", payload)
}

func (c *Client) CreateTokenSplitPayment(ctx context.Context, amount float64, orderID, splitAccount string, splitAmount float64, opts *PaymentOptions) (*Response, error) {
	payload := c.checkout(amount, orderID, opts)
	payload["split_user"] = splitAccount
	payload["split_amount"] = money(splitAmount)
	return c.post(ctx, "/api/1/token/split-payment", payload)
}

func (c *Client) SplitChargeSavedCard(ctx context.Context, cardID string, amount float64, orderID, splitAccount string, splitAmount float64, opts *ChargeOptions) (*Response, error) {
	if opts == nil {
		opts = &ChargeOptions{}
	}
	return c.post(ctx, "/api/1/split-execute-pay", map[string]any{
		"card_id":      cardID,
		"amount":       money(amount),
		"currency":     orDefault(opts.Currency, c.currency),
		"language":     orDefault(opts.Language, c.language),
		"order_id":     orderID,
		"split_user":   splitAccount,
		"split_amount": money(splitAmount),
		"description":  opts.Description,
	})
}

func (c *Client) RegisterCard(ctx context.Context, opts *RegisterCardOptions) (*Response, error) {
	if opts == nil {
		opts = &RegisterCardOptions{}
	}
	return c.post(ctx, "/api/1/card-registration", map[string]any{
		"language":             orDefault(opts.Language, c.language),
		"refund":               boolInt(opts.ForPayouts),
		"description":          opts.Description,
		"success_redirect_url": orDefault(opts.SuccessRedirectURL, c.successRedirectURL),
		"error_redirect_url":   orDefault(opts.ErrorRedirectURL, c.errorRedirectURL),
	})
}

func (c *Client) RegisterCardAndPay(ctx context.Context, amount float64, orderID string, opts *PaymentOptions) (*Response, error) {
	return c.post(ctx, "/api/1/card-registration-with-pay", c.checkout(amount, orderID, opts))
}

func (c *Client) ChargeSavedCard(ctx context.Context, cardID string, amount float64, orderID string, opts *ChargeOptions) (*Response, error) {
	if opts == nil {
		opts = &ChargeOptions{}
	}
	return c.post(ctx, "/api/1/execute-pay", map[string]any{
		"card_id":     cardID,
		"amount":      money(amount),
		"currency":    orDefault(opts.Currency, c.currency),
		"language":    orDefault(opts.Language, c.language),
		"order_id":    orderID,
		"description": opts.Description,
	})
}

func (c *Client) Refund(ctx context.Context, cardID string, amount float64, orderID string, opts *ChargeOptions) (*Response, error) {
	if opts == nil {
		opts = &ChargeOptions{}
	}
	return c.post(ctx, "/api/1/refund-request", map[string]any{
		"card_id":     cardID,
		"amount":      money(amount),
		"currency":    orDefault(opts.Currency, c.currency),
		"language":    orDefault(opts.Language, c.language),
		"order_id":    orderID,
		"description": opts.Description,
	})
}

func (c *Client) Reverse(ctx context.Context, transaction string, opts *ReverseOptions) (*Response, error) {
	if opts == nil {
		opts = &ReverseOptions{}
	}
	payload := map[string]any{
		"transaction": transaction,
		"currency":    orDefault(opts.Currency, c.currency),
		"language":    orDefault(opts.Language, c.language),
	}
	if opts.Amount != nil {
		payload["amount"] = money(*opts.Amount)
	}
	return c.post(ctx, "/api/1/reverse", payload)
}

func (c *Client) GetInstallmentPlans(ctx context.Context, orderID string, opts *InstallmentQueryOptions) ([]map[string]any, error) {
	if opts == nil {
		opts = &InstallmentQueryOptions{}
	}
	payload := map[string]any{
		"order_id":    orderID,
		"currency":    orDefault(opts.Currency, c.currency),
		"language":    orDefault(opts.Language, c.language),
		"description": opts.Description,
		"other_attr":  opts.OtherAttr,
	}
	if opts.Amount != nil {
		payload["amount"] = money(*opts.Amount)
	}
	result, err := c.postRaw(ctx, "/api/1/get-installment-request", payload)
	if err != nil {
		return nil, err
	}
	items, ok := result.([]any)
	if !ok {
		return nil, nil
	}
	plans := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if plan, ok := item.(map[string]any); ok {
			plans = append(plans, plan)
		}
	}
	return plans, nil
}

func (c *Client) PayByInstallment(ctx context.Context, amount float64, orderID, installmentCardID string, installmentMonth int, opts *PaymentOptions) (*Response, error) {
	payload := c.checkout(amount, orderID, opts)
	payload["installment_card_id"] = installmentCardID
	payload["installment_month"] = installmentMonth
	return c.post(ctx, "/api/1/installment-request", payload)
}

func (c *Client) ListWallets(ctx context.Context) (map[string]string, error) {
	result, err := c.postRaw(ctx, "/api/1/wallet/status", map[string]any{})
	if err != nil {
		return nil, err
	}
	raw, ok := result.(map[string]any)
	if !ok {
		return map[string]string{}, nil
	}
	wallets := make(map[string]string, len(raw))
	for key, value := range raw {
		if s, ok := value.(string); ok {
			wallets[key] = s
		}
	}
	return wallets, nil
}

func (c *Client) PayWithWallet(ctx context.Context, walletID string, amount float64, orderID string, opts *ChargeOptions) (*Response, error) {
	if opts == nil {
		opts = &ChargeOptions{}
	}
	return c.post(ctx, "/api/1/wallet/payment", map[string]any{
		"wallet_id":   walletID,
		"amount":      money(amount),
		"currency":    orDefault(opts.Currency, c.currency),
		"language":    orDefault(opts.Language, c.language),
		"order_id":    orderID,
		"description": opts.Description,
	})
}

func (c *Client) CreateWidget(ctx context.Context, amount float64, orderID, description string) (*Response, error) {
	return c.post(ctx, "/api/1/token/widget", map[string]any{
		"amount":      money(amount),
		"order_id":    orderID,
		"description": description,
	})
}

func (c *Client) CreateInvoice(ctx context.Context, amount float64, periodFrom, periodTo string, opts *InvoiceOptions) (*Response, error) {
	return c.post(ctx, "/api/1/invoices/create", c.invoice(amount, periodFrom, periodTo, opts))
}

func (c *Client) UpdateInvoice(ctx context.Context, invoiceID int, amount float64, periodFrom, periodTo string, opts *InvoiceOptions) (*Response, error) {
	payload := c.invoice(amount, periodFrom, periodTo, opts)
	payload["id"] = invoiceID
	return c.post(ctx, "/api/1/invoices/update", payload)
}

func (c *Client) invoice(amount float64, periodFrom, periodTo string, opts *InvoiceOptions) map[string]any {
	if opts == nil {
		opts = &InvoiceOptions{}
	}
	return map[string]any{
		"sum":                money(amount),
		"period_from":        periodFrom,
		"period_to":          periodTo,
		"name":               opts.Name,
		"description":        opts.Description,
		"phone":              opts.Phone,
		"email":              opts.Email,
		"inn":                opts.TIN,
		"contract_number":    opts.ContractNumber,
		"merchant_order_id":  opts.MerchantOrderID,
		"display":            boolInt(!opts.Hidden),
		"save_as_template":   boolInt(opts.SaveAsTemplate),
		"status_installment": boolInt(opts.AllowInstallment),
	}
}

func (c *Client) GetInvoice(ctx context.Context, invoiceID int) (*Response, error) {
	return c.post(ctx, "/api/1/invoices/view", map[string]any{"id": invoiceID})
}

func (c *Client) ListInvoices(ctx context.Context, kind, order string) (*Response, error) {
	return c.post(ctx, "/api/1/invoices/list", map[string]any{"type": kind, "order": order})
}

func (c *Client) SendInvoiceSMS(ctx context.Context, invoiceID int, phone string) (*Response, error) {
	return c.post(ctx, "/api/1/invoices/send-sms", map[string]any{"id": invoiceID, "phone": phone})
}

func (c *Client) SendInvoiceEmail(ctx context.Context, invoiceID int, email string) (*Response, error) {
	return c.post(ctx, "/api/1/invoices/send-email", map[string]any{"id": invoiceID, "email": email})
}

func (c *Client) CreateBankTransfer(ctx context.Context, transfer BankTransfer) (*Response, error) {
	return c.postJSON(ctx, "/api/1/b2b/payment", map[string]any{
		"order_id":         transfer.OrderID,
		"amount":           money(transfer.Amount),
		"description":      transfer.Description,
		"iban":             transfer.IBAN,
		"name":             transfer.Name,
		"tin":              transfer.TIN,
		"bank_code":        transfer.BankCode,
		"bank_name":        transfer.BankName,
		"email":            transfer.Email,
		"address":          transfer.Address,
		"additional_info":  transfer.AdditionalInfo,
		"webhook_url":      transfer.WebhookURL,
		"bulk_description": transfer.BulkDescription,
	})
}

func (c *Client) GetBankTransfer(ctx context.Context, orderID string) (*Response, error) {
	status, parsed, err := c.send(ctx, http.MethodGet, "/api/1/b2b/payment/"+orderID, nil, "")
	if err != nil {
		return nil, err
	}
	return parse(status, parsed)
}

func (c *Client) Heartbeat(ctx context.Context) (*Response, error) {
	status, parsed, err := c.send(ctx, http.MethodGet, "/api/heartbeat", nil, "")
	if err != nil {
		return nil, err
	}
	return parse(status, parsed)
}

func (c *Client) VerifyCallback(data, signature string) (*Callback, error) {
	return VerifyCallback(c.privateKey, data, signature)
}
