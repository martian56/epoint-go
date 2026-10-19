package epoint_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// setFeatures flips merchant capabilities and waits until the API agrees.
//
// The PATCH response cannot be trusted on its own: the sandbox serialises the
// merchant after flushing but before the transaction commits, so it reports the
// new value while a request arriving straight after can still read the old one.
// Reading it back in a separate request is what confirms the change landed.
func setFeatures(t *testing.T, features map[string]bool) {
	t.Helper()
	body, _ := json.Marshal(features)

	if _, err := requestMerchant(http.MethodPatch, body); err != nil {
		t.Fatalf("could not set %v on %s: %v", features, sandboxPublicKey, err)
	}

	for attempt := 0; attempt < 40; attempt++ {
		merchant, err := readMerchant()
		if err == nil && featuresMatch(merchant, features) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%s never reported %v", sandboxPublicKey, features)
}

func requestMerchant(method string, body []byte) (map[string]any, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, sandboxURL()+"/_sandbox/merchants/"+sandboxPublicKey, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("the sandbox answered %d", resp.StatusCode)
	}

	var merchant map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&merchant); err != nil {
		return nil, err
	}
	return merchant, nil
}

// readMerchant goes through the list, because there is no route for a single
// merchant: /_sandbox/merchants/{key} is only a PATCH, and a GET there falls
// through to the dashboard and answers with HTML.
func readMerchant() (map[string]any, error) {
	resp, err := http.Get(sandboxURL() + "/_sandbox/merchants")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("the sandbox answered %d", resp.StatusCode)
	}

	var merchants []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&merchants); err != nil {
		return nil, err
	}
	for _, merchant := range merchants {
		if merchant["public_key"] == sandboxPublicKey {
			return merchant, nil
		}
	}
	return nil, fmt.Errorf("%s is not in the merchant list", sandboxPublicKey)
}

func featuresMatch(merchant map[string]any, want map[string]bool) bool {
	features, ok := merchant["features"].(map[string]any)
	if !ok {
		return false
	}
	for name, expected := range want {
		if got, ok := features[name].(bool); !ok || got != expected {
			return false
		}
	}
	return true
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

func TestSandboxCardRegistrationReturnsToTheRedirectURLs(t *testing.T) {
	requireSandbox(t)
	client := newSandboxClient()
	ctx := context.Background()
	opts := &epoint.RegisterCardOptions{
		Description:        "redirect check",
		SuccessRedirectURL: "https://shop.test/card-ok",
		ErrorRedirectURL:   "https://shop.test/card-fail",
	}
	approved, err := client.RegisterCard(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	declined, err := client.RegisterCard(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}

	submit := func(redirectURL, card string) *http.Response {
		parts := strings.Split(strings.TrimRight(redirectURL, "/"), "/")
		token := strings.Split(parts[len(parts)-1], "?")[0]
		web := &http.Client{
			Timeout:       10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
		resp, err := web.PostForm(sandboxURL()+"/checkout/"+token, url.Values{
			"card_number": {card},
			"card_holder": {"TESTER"},
			"month":       {"12"},
			"year":        {"30"},
		})
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp
	}

	ok := submit(approved.RedirectURL(), "4111111111111111")
	failed := submit(declined.RedirectURL(), "4000000000000116")
	if ok.StatusCode != http.StatusSeeOther || ok.Header.Get("Location") != "https://shop.test/card-ok" {
		t.Fatalf("approved card went to %d %q", ok.StatusCode, ok.Header.Get("Location"))
	}
	if failed.StatusCode != http.StatusSeeOther || failed.Header.Get("Location") != "https://shop.test/card-fail" {
		t.Fatalf("declined card went to %d %q", failed.StatusCode, failed.Header.Get("Location"))
	}

	cardID := approved.String("card_id")
	active := waitFor(t, func() (*epoint.Response, bool) {
		status, err := client.GetCardStatus(ctx, cardID)
		return status, err == nil && status.String("status") == "active"
	})
	if active.String("status") != "active" {
		t.Fatalf("card not active: %+v", active.Raw)
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
