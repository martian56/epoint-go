package epoint_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	epoint "github.com/martian56/epoint-go"
)

func paidStatus(t *testing.T, card string) *epoint.Response {
	t.Helper()
	client := newSandboxClient()
	created, err := client.CreatePayment(context.Background(), 10, orderID(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	pay(t, created.RedirectURL(), card)
	return settled(t, client, created.Transaction())
}

func TestSandboxPaymentEndingInErrorReadsBack(t *testing.T) {
	requireSandbox(t)
	status := paidStatus(t, "4000000000009998")
	if status.Status() != "error" || status.OK() {
		t.Fatalf("expected status error, got %+v", status.Raw)
	}
	if status.Message() != "An error occurred during payment" || status.Code() != "" {
		t.Fatalf("unexpected error state: %+v", status.Raw)
	}
	if status.Transaction() == "" || status.CardMask() != "400000******9998" {
		t.Fatalf("unexpected error state: %+v", status.Raw)
	}
}

func TestSandboxUnknownTransactionIsStillAnError(t *testing.T) {
	requireSandbox(t)
	_, err := newSandboxClient().GetStatus(context.Background(), "te999999999999")
	var gwErr *epoint.GatewayError
	if !errors.As(err, &gwErr) {
		t.Fatalf("expected GatewayError, got %v", err)
	}
}

func TestSandboxServerErrorReadsBack(t *testing.T) {
	requireSandbox(t)
	status := paidStatus(t, "4000000000009999")
	if status.Status() != "server_error" || status.OK() {
		t.Fatalf("expected server_error, got %+v", status.Raw)
	}
}

func TestSandboxReturnedReadsBack(t *testing.T) {
	requireSandbox(t)
	client := newSandboxClient()
	ctx := context.Background()

	created, err := client.CreatePayment(ctx, 12, orderID(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	pay(t, created.RedirectURL(), "4111111111111111")
	settled(t, client, created.Transaction())
	if _, err := client.Reverse(ctx, created.Transaction(), nil); err != nil {
		t.Fatal(err)
	}

	status := waitFor(t, func() (*epoint.Response, bool) {
		status, err := client.GetStatus(ctx, created.Transaction())
		return status, err == nil && status.Status() == "returned"
	})
	if status.OK() || status.Amount() != 12 || status.TraceID() == "" {
		t.Fatalf("unexpected returned state: %+v", status.Raw)
	}
}

func TestSandboxTokenSplitPayment(t *testing.T) {
	requireSandbox(t)
	setFeatures(t, map[string]bool{"token_payments_enabled": true})
	client := newSandboxClient()
	ctx := context.Background()

	created, err := client.CreateTokenSplitPayment(ctx, 100, orderID(t), partnerKey, 30, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !created.OK() || created.RedirectURL() == "" || created.Transaction() == "" || created.TraceID() == "" {
		t.Fatalf("token split not created: %+v", created.Raw)
	}
	pay(t, created.RedirectURL(), "4111111111111111")

	status := settled(t, client, created.Transaction())
	if status.Status() != "success" || !status.OK() {
		t.Fatalf("token split not settled: %+v", status.Raw)
	}

	_, err = client.CreateTokenSplitPayment(ctx, 100, orderID(t), "i999999999", 30, nil)
	var gwErr *epoint.GatewayError
	if !errors.As(err, &gwErr) {
		t.Fatalf("expected GatewayError for an unknown split_user, got %v", err)
	}
}

func TestSandboxCheckoutForm(t *testing.T) {
	requireSandbox(t)
	client := newSandboxClient()
	yes := true

	order := orderID(t)
	form, err := client.CheckoutForm(18.4, order, &epoint.PaymentOptions{Description: "form post", IsInstallment: &yes})
	if err != nil {
		t.Fatal(err)
	}
	web := &http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := web.PostForm(form.URL, url.Values{"data": {form.Data}, "signature": {form.Signature}})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	location := resp.Header.Get("Location")
	if resp.StatusCode != http.StatusSeeOther || !strings.Contains(location, "/checkout/") {
		t.Fatalf("expected 303 to the card page, got %d %q", resp.StatusCode, location)
	}

	cardPage, err := url.Parse(location)
	if err != nil {
		t.Fatal(err)
	}
	pay(t, cardPage.Path, "4111111111111111")

	transaction := waitFor(t, func() (string, bool) {
		value, err := transactionForOrder(order)
		return value, err == nil && value != ""
	})
	status := settled(t, client, transaction)
	if status.Status() != "success" || status.Amount() != 18.4 {
		t.Fatalf("form payment not settled: %+v", status.Raw)
	}
}

func transactionForOrder(order string) (string, error) {
	resp, err := http.Get(sandboxURL() + "/_sandbox/transactions?limit=200&merchant=" + sandboxPublicKey)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var rows []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return "", err
	}
	for _, row := range rows {
		if row["order_id"] == order {
			if transaction, ok := row["transaction"].(string); ok {
				return transaction, nil
			}
		}
	}
	return "", nil
}

func TestSandboxNewOptionalParametersAreAccepted(t *testing.T) {
	requireSandbox(t)
	grantAll(t)
	client := newSandboxClient()
	ctx := context.Background()
	no := false

	created, err := client.CreatePayment(ctx, 5, orderID(t), &epoint.PaymentOptions{IsInstallment: &no})
	if err != nil || !created.OK() {
		t.Fatalf("payment with is_installment refused: %v", err)
	}
	request, err := client.CreatePaymentRequest(ctx, 5, orderID(t), &epoint.PaymentOptions{IsInstallment: &no})
	if err != nil || !request.OK() {
		t.Fatalf("payment request with is_installment refused: %v", err)
	}

	amount := 300.0
	plans, err := client.GetInstallmentPlans(ctx, orderID(t), &epoint.InstallmentQueryOptions{
		Amount:      &amount,
		Description: "sofa",
		OtherAttr:   map[string]any{"ref": "x"},
	})
	if err != nil || len(plans) == 0 {
		t.Fatalf("installment plans with description refused: %v %v", plans, err)
	}

	transfer, err := client.CreateBankTransfer(ctx, epoint.BankTransfer{
		OrderID:         orderID(t),
		Amount:          75,
		Description:     "fees",
		IBAN:            "AZ21NABZ00000000137010001944",
		Name:            "Logistika MMC",
		TIN:             "1234567892",
		BankCode:        "505141",
		BulkDescription: "July payroll",
	})
	if err != nil || !transfer.OK() {
		t.Fatalf("bank transfer with bulk_description refused: %v", err)
	}
}

func TestSandboxInvoiceUpdateFields(t *testing.T) {
	requireSandbox(t)
	client := newSandboxClient()
	ctx := context.Background()

	created, err := client.CreateInvoice(ctx, 30, "2026-01-01", "2026-12-31", &epoint.InvoiceOptions{Name: "Murad"})
	if err != nil {
		t.Fatal(err)
	}
	invoiceID := int(created.Get("id").(float64))
	waitFor(t, func() (*epoint.Response, bool) {
		invoice, err := client.GetInvoice(ctx, invoiceID)
		return invoice, err == nil
	})

	_, err = client.UpdateInvoice(ctx, invoiceID, 35, "2026-01-01", "2026-12-31", &epoint.InvoiceOptions{
		Name:            "Murad",
		TIN:             "1234567892",
		ContractNumber:  "C-2026-7",
		MerchantOrderID: "shop-77",
	})
	if err != nil {
		t.Fatal(err)
	}

	invoice := waitFor(t, func() (map[string]any, bool) {
		view, err := client.GetInvoice(ctx, invoiceID)
		if err != nil {
			return nil, false
		}
		fields, ok := view.Get("invoice").(map[string]any)
		return fields, ok && fields["merchant_order_id"] == "shop-77"
	})
	if invoice["inn"] != "1234567892" || invoice["contract_number"] != "C-2026-7" {
		t.Fatalf("invoice fields not returned: %+v", invoice)
	}
}
