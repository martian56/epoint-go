package epoint

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetStatusReturnsAPaymentThatEndedInError(t *testing.T) {
	server, _ := recording(t, 200, map[string]any{
		"status":      "error",
		"code":        "",
		"message":     "An error occurred during payment",
		"transaction": "te1",
	})
	result, err := clientFor(server).GetStatus(context.Background(), "te1")
	if err != nil {
		t.Fatalf("a payment in status error should be returned: %v", err)
	}
	if result.Status() != "error" || result.Transaction() != "te1" || result.OK() {
		t.Fatalf("unexpected result: %+v", result.Raw)
	}
}

func TestGetStatusErrorEnvelopeWithoutTransactionIsGatewayError(t *testing.T) {
	for _, body := range []map[string]any{
		{"status": "error", "message": "Unknown transaction: te9"},
		{"status": "error", "message": "Unknown transaction: te9", "transaction": ""},
		{"status": "error", "message": "Unknown transaction: te9", "transaction": nil},
	} {
		server, _ := recording(t, 200, body)
		_, err := clientFor(server).GetStatus(context.Background(), "te9")
		var gwErr *GatewayError
		if !errors.As(err, &gwErr) {
			t.Fatalf("expected GatewayError for %v, got %v", body, err)
		}
	}
}

func TestGetStatusNon2xxWithTransactionStillErrors(t *testing.T) {
	server, _ := recording(t, 500, map[string]any{"status": "error", "transaction": "te1", "message": "boom"})
	_, err := clientFor(server).GetStatus(context.Background(), "te1")
	var gwErr *GatewayError
	if !errors.As(err, &gwErr) {
		t.Fatalf("expected GatewayError, got %v", err)
	}
}

func TestOtherEndpointsStillErrorOnStatusErrorWithTransaction(t *testing.T) {
	server, _ := recording(t, 200, map[string]any{"status": "error", "transaction": "te1", "message": "refused"})
	_, err := clientFor(server).CreatePayment(context.Background(), 1, "o", nil)
	var gwErr *GatewayError
	if !errors.As(err, &gwErr) {
		t.Fatalf("expected GatewayError, got %v", err)
	}
}

func TestTokenSplitPaymentPayload(t *testing.T) {
	var path string
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_ = r.ParseForm()
		decoded, _ := base64.StdEncoding.DecodeString(r.FormValue("data"))
		_ = json.Unmarshal(decoded, &payload)
		_, _ = w.Write([]byte(`{"status":"success","transaction":"te1","redirect_url":"https://pay.test/checkout/x"}`))
	}))
	t.Cleanup(server.Close)
	client := New(testPublicKey, testMerchantPK, WithBaseURL(server.URL),
		WithRedirectURLs("https://shop.test/ok", "https://shop.test/fail"))

	result, err := client.CreateTokenSplitPayment(context.Background(), 100, "o1", "i000000002", 30, &PaymentOptions{
		Description:      "split",
		ErrorRedirectURL: "https://shop.test/own-fail",
	})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/api/1/token/split-payment" {
		t.Fatalf("posted to %s", path)
	}
	if result.Transaction() != "te1" || result.RedirectURL() == "" {
		t.Fatalf("unexpected result: %+v", result.Raw)
	}
	want := map[string]any{
		"public_key":           testPublicKey,
		"amount":               "100.00",
		"currency":             "AZN",
		"language":             "az",
		"order_id":             "o1",
		"description":          "split",
		"split_user":           "i000000002",
		"split_amount":         "30.00",
		"success_redirect_url": "https://shop.test/ok",
		"error_redirect_url":   "https://shop.test/own-fail",
	}
	assertPayload(t, payload, want)
}

func TestCheckoutFormSignsWithoutARequest(t *testing.T) {
	yes := true
	client := New(testPublicKey, testMerchantPK, WithBaseURL("https://pay.test/"),
		WithRedirectURLs("https://shop.test/ok", "https://shop.test/fail"))
	form, err := client.CheckoutForm(30.75, "o1", &PaymentOptions{Description: "Kilim", IsInstallment: &yes})
	if err != nil {
		t.Fatal(err)
	}
	if form.URL != "https://pay.test/api/1/checkout" {
		t.Fatalf("url = %s", form.URL)
	}
	if !signatureMatches(testMerchantPK, form.Data, form.Signature) {
		t.Fatal("signature did not verify")
	}
	payload, err := decodeData(form.Data)
	if err != nil {
		t.Fatal(err)
	}
	assertPayload(t, payload, map[string]any{
		"public_key":           testPublicKey,
		"amount":               "30.75",
		"currency":             "AZN",
		"language":             "az",
		"order_id":             "o1",
		"description":          "Kilim",
		"success_redirect_url": "https://shop.test/ok",
		"error_redirect_url":   "https://shop.test/fail",
		"is_installment":       float64(1),
	})
}

func TestIsInstallmentIsSentAsAnInteger(t *testing.T) {
	yes, no := true, false
	server, seen := recording(t, 200, map[string]any{"status": "success"})
	client := clientFor(server)
	ctx := context.Background()

	if _, err := client.CreatePayment(ctx, 1, "o", nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := seen.payload["is_installment"]; ok {
		t.Fatalf("is_installment sent without being given: %v", seen.payload)
	}
	if _, err := client.CreatePayment(ctx, 1, "o", &PaymentOptions{IsInstallment: &yes}); err != nil {
		t.Fatal(err)
	}
	if seen.payload["is_installment"] != float64(1) {
		t.Fatalf("is_installment = %v", seen.payload["is_installment"])
	}
	if _, err := client.CreatePaymentRequest(ctx, 1, "o", &PaymentOptions{IsInstallment: &no}); err != nil {
		t.Fatal(err)
	}
	if seen.payload["is_installment"] != float64(0) {
		t.Fatalf("is_installment = %v", seen.payload["is_installment"])
	}
	if _, err := client.CreatePaymentRequest(ctx, 1, "o", &PaymentOptions{IsInstallment: &yes}); err != nil {
		t.Fatal(err)
	}
	if seen.payload["is_installment"] != float64(1) {
		t.Fatalf("is_installment = %v", seen.payload["is_installment"])
	}
	form, err := client.CheckoutForm(1, "o", nil)
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := decodeData(form.Data)
	if _, ok := payload["is_installment"]; ok {
		t.Fatalf("is_installment sent without being given: %v", payload)
	}
}

func TestBankTransferSendsBulkDescription(t *testing.T) {
	server, seen := recording(t, 200, map[string]any{"status": "success"})
	_, err := clientFor(server).CreateBankTransfer(context.Background(), BankTransfer{
		OrderID:         "b1",
		Amount:          10,
		BulkDescription: "July payroll",
	})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := decodeData(seen.jsonBody["data"])
	if err != nil {
		t.Fatal(err)
	}
	if payload["bulk_description"] != "July payroll" {
		t.Fatalf("bulk_description = %v", payload["bulk_description"])
	}
}

func TestInstallmentPlansSendDescriptionAndOtherAttr(t *testing.T) {
	server, seen := recording(t, 200, []any{})
	_, err := clientFor(server).GetInstallmentPlans(context.Background(), "o1", &InstallmentQueryOptions{
		Description: "sofa",
		OtherAttr:   map[string]any{"ref": "x"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen.payload["description"] != "sofa" {
		t.Fatalf("description = %v", seen.payload["description"])
	}
	attr, ok := seen.payload["other_attr"].(map[string]any)
	if !ok || attr["ref"] != "x" {
		t.Fatalf("other_attr = %v", seen.payload["other_attr"])
	}
}

func TestUpdateInvoicePayload(t *testing.T) {
	server, seen := recording(t, 200, map[string]any{"status": "success"})
	_, err := clientFor(server).UpdateInvoice(context.Background(), 7, 20, "2026-01-01", "2026-02-01", &InvoiceOptions{
		TIN:             "1234567892",
		ContractNumber:  "C-1",
		MerchantOrderID: "m-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen.payload["inn"] != "1234567892" || seen.payload["contract_number"] != "C-1" ||
		seen.payload["merchant_order_id"] != "m-1" || seen.payload["id"] != float64(7) {
		t.Fatalf("payload = %v", seen.payload)
	}
}

func assertPayload(t *testing.T, payload, want map[string]any) {
	t.Helper()
	if len(payload) != len(want) {
		t.Fatalf("payload = %v, want %v", payload, want)
	}
	for key, value := range want {
		if payload[key] != value {
			t.Fatalf("%s = %v, want %v", key, payload[key], value)
		}
	}
}
