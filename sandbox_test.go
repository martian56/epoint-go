package epoint_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	epoint "github.com/martian56/epoint-go"
)

const (
	sandboxPublicKey = "i000000001"
	sandboxSecret    = "sandbox_private_key_0000000001"
	partnerKey       = "i000000002"
)

func sandboxURL() string {
	if u := os.Getenv("EPOINT_BASE_URL"); u != "" {
		return u
	}
	return "http://localhost:8181"
}

func newSandboxClient() *epoint.Client {
	return epoint.New(sandboxPublicKey, sandboxSecret,
		epoint.WithBaseURL(sandboxURL()), epoint.WithLanguage("en"))
}

func requireSandbox(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping sandbox test in short mode")
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(sandboxURL() + "/api/heartbeat")
	if err != nil {
		t.Skipf("no sandbox on %s", sandboxURL())
	}
	resp.Body.Close()
}

func orderID(t *testing.T) string {
	t.Helper()
	buf := make([]byte, 6)
	_, _ = rand.Read(buf)
	return "go-" + hex.EncodeToString(buf)
}

// waitFor polls until fn reports success. The sandbox returns a create response
// before its transaction commits, so a machine-speed read that follows can miss
// it by a few milliseconds. A browser redirect never hits this, but a test does.
func waitFor[T any](t *testing.T, fn func() (T, bool)) T {
	t.Helper()
	for range 25 {
		if value, ok := fn(); ok {
			return value
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("condition was not met in time")
	var zero T
	return zero
}

func settled(t *testing.T, client *epoint.Client, transaction string) *epoint.Response {
	t.Helper()
	return waitFor(t, func() (*epoint.Response, bool) {
		status, err := client.GetStatus(context.Background(), transaction)
		return status, err == nil && status.Status() != "new"
	})
}

func pay(t *testing.T, redirectURL, card string) {
	t.Helper()
	parts := strings.Split(strings.TrimRight(redirectURL, "/"), "/")
	token := strings.Split(parts[len(parts)-1], "?")[0]
	form := url.Values{
		"card_number": {card},
		"card_holder": {"TESTER"},
		"month":       {"12"},
		"year":        {"30"},
	}
	client := &http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	for range 25 {
		resp, err := client.PostForm(sandboxURL()+"/checkout/"+token, form)
		if err != nil {
			t.Fatal(err)
		}
		status := resp.StatusCode
		resp.Body.Close()
		if status != http.StatusNotFound {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("checkout token %s never became available", token)
}

func grantAll(t *testing.T) {
	t.Helper()
	setFeatures(t, map[string]bool{
		"amex_enabled":           true,
		"token_payments_enabled": true,
		"installments_enabled":   true,
		"wallets_enabled":        true,
		"b2b_enabled":            true,
	})
}

func setFeatures(t *testing.T, features map[string]bool) {
	t.Helper()
	body, _ := json.Marshal(features)
	req, _ := http.NewRequest(http.MethodPatch,
		sandboxURL()+"/_sandbox/merchants/"+sandboxPublicKey, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func TestSandboxHeartbeat(t *testing.T) {
	requireSandbox(t)
	resp, err := newSandboxClient().Heartbeat(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if resp.String("status") != "ok" {
		t.Fatalf("unexpected heartbeat: %+v", resp.Raw)
	}
}

func TestSandboxPaymentEndToEnd(t *testing.T) {
	requireSandbox(t)
	client := newSandboxClient()
	ctx := context.Background()

	created, err := client.CreatePayment(ctx, 25.5, orderID(t), &epoint.PaymentOptions{Description: "Kilim"})
	if err != nil {
		t.Fatal(err)
	}
	if !created.OK() || created.RedirectURL() == "" {
		t.Fatalf("payment not created: %+v", created.Raw)
	}
	pay(t, created.RedirectURL(), "4111111111111111")

	status := settled(t, client, created.Transaction())
	if status.Status() != "success" || status.String("code") != "000" {
		t.Fatalf("unexpected status: %+v", status.Raw)
	}
	if status.String("card_mask") != "411111******1111" {
		t.Fatalf("unexpected mask: %s", status.String("card_mask"))
	}
}

func TestSandboxDeclinedCard(t *testing.T) {
	requireSandbox(t)
	client := newSandboxClient()
	ctx := context.Background()

	created, err := client.CreatePayment(ctx, 10, orderID(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	pay(t, created.RedirectURL(), "4000000000000116")

	status := settled(t, client, created.Transaction())
	if status.Status() != "failed" || status.String("code") != "116" {
		t.Fatalf("expected failed 116, got %+v", status.Raw)
	}
}

func TestSandboxSavedCard(t *testing.T) {
	requireSandbox(t)
	client := newSandboxClient()
	ctx := context.Background()

	reg, err := client.RegisterCard(ctx, &epoint.RegisterCardOptions{Description: "repeat"})
	if err != nil {
		t.Fatal(err)
	}
	cardID := reg.String("card_id")
	if cardID == "" {
		t.Fatal("no card_id returned")
	}
	pay(t, reg.RedirectURL(), "4111111111111111")

	active := waitFor(t, func() (*epoint.Response, bool) {
		status, err := client.GetCardStatus(ctx, cardID)
		return status, err == nil && status.String("status") == "active"
	})
	if active.String("status") != "active" {
		t.Fatalf("card not active: %+v", active.Raw)
	}

	charge, err := client.ChargeSavedCard(ctx, cardID, 15, orderID(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	if charge.String("bank_response") != "000" {
		t.Fatalf("charge not approved: %+v", charge.Raw)
	}
}

func TestSandboxReverse(t *testing.T) {
	requireSandbox(t)
	client := newSandboxClient()
	ctx := context.Background()

	created, err := client.CreatePayment(ctx, 40, orderID(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	pay(t, created.RedirectURL(), "4111111111111111")
	settled(t, client, created.Transaction())

	reversed, err := client.Reverse(ctx, created.Transaction(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reversed.OK() {
		t.Fatalf("reverse failed: %+v", reversed.Raw)
	}
}

func TestSandboxSplitPayment(t *testing.T) {
	requireSandbox(t)
	client := newSandboxClient()
	ctx := context.Background()

	created, err := client.CreateSplitPayment(ctx, 100, orderID(t), partnerKey, 30, nil)
	if err != nil {
		t.Fatal(err)
	}
	pay(t, created.RedirectURL(), "4111111111111111")

	status := settled(t, client, created.Transaction())
	if status.Status() != "success" {
		t.Fatalf("split not successful: %+v", status.Raw)
	}
}

func TestSandboxUngrantedFeatureIsRefused(t *testing.T) {
	requireSandbox(t)
	setFeatures(t, map[string]bool{"wallets_enabled": false})

	_, err := newSandboxClient().PayWithWallet(context.Background(), "wallet_epul", 10, orderID(t), nil)
	var gwErr *epoint.GatewayError
	if !errors.As(err, &gwErr) {
		t.Fatalf("expected GatewayError, got %v", err)
	}
}

func TestSandboxWalletsAndInstallments(t *testing.T) {
	requireSandbox(t)
	grantAll(t)
	client := newSandboxClient()
	ctx := context.Background()

	wallets, err := client.ListWallets(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(wallets) == 0 {
		t.Fatal("no wallets returned")
	}

	amount := 200.0
	plans, err := client.GetInstallmentPlans(ctx, orderID(t), &epoint.InstallmentQueryOptions{Amount: &amount})
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) == 0 || plans[0]["installment_card_id"] == nil {
		t.Fatalf("unexpected plans: %+v", plans)
	}
}

func TestSandboxWidget(t *testing.T) {
	requireSandbox(t)
	grantAll(t)
	widget, err := newSandboxClient().CreateWidget(context.Background(), 50, orderID(t), "wallet")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(widget.String("widget_url"), "widget=1") {
		t.Fatalf("unexpected widget url: %s", widget.String("widget_url"))
	}
}

func TestSandboxInvoiceLifecycle(t *testing.T) {
	requireSandbox(t)
	client := newSandboxClient()
	ctx := context.Background()

	created, err := client.CreateInvoice(ctx, 49.9, "2026-01-01", "2026-12-31", &epoint.InvoiceOptions{
		Name:        "Leyla",
		Description: "deposit",
		Phone:       "+994501234567",
		Email:       "leyla@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	invoiceID := int(created.Get("id").(float64))

	waitFor(t, func() (*epoint.Response, bool) {
		invoice, err := client.GetInvoice(ctx, invoiceID)
		return invoice, err == nil
	})
	if _, err := client.SendInvoiceSMS(ctx, invoiceID, "+994501234567"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.SendInvoiceEmail(ctx, invoiceID, "leyla@example.com"); err != nil {
		t.Fatal(err)
	}
}

func TestSandboxBankTransfer(t *testing.T) {
	requireSandbox(t)
	grantAll(t)
	client := newSandboxClient()
	ctx := context.Background()
	id := orderID(t)

	created, err := client.CreateBankTransfer(ctx, epoint.BankTransfer{
		OrderID:     id,
		Amount:      1250,
		Description: "July fees",
		IBAN:        "AZ21NABZ00000000137010001944",
		Name:        "Logistika MMC",
		TIN:         "1234567892",
		BankCode:    "505141",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created.OK() {
		t.Fatalf("transfer not accepted: %+v", created.Raw)
	}

	lookup := waitFor(t, func() (*epoint.Response, bool) {
		transfer, err := client.GetBankTransfer(ctx, id)
		return transfer, err == nil
	})
	if lookup.String("order_id") != id {
		t.Fatalf("unexpected lookup: %+v", lookup.Raw)
	}
}

func TestSandboxMissingRequiredField(t *testing.T) {
	requireSandbox(t)
	_, err := newSandboxClient().CreatePayment(context.Background(), 10, "", nil)
	var gwErr *epoint.GatewayError
	if !errors.As(err, &gwErr) {
		t.Fatalf("expected GatewayError, got %v", err)
	}
}
