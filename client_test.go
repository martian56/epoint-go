package epoint

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	testPublicKey  = "i000000001"
	testMerchantPK = "sandbox_private_key_0000000001"
)

type capture struct {
	contentType string
	form        map[string]string
	payload     map[string]any
	jsonBody    map[string]string
}

func recording(t *testing.T, status int, body any) (*httptest.Server, *capture) {
	t.Helper()
	seen := &capture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.contentType = r.Header.Get("Content-Type")
		if seen.contentType == "application/json" {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &seen.jsonBody)
		} else {
			_ = r.ParseForm()
			data := r.FormValue("data")
			seen.form = map[string]string{"data": data, "signature": r.FormValue("signature")}
			if decoded, err := base64.StdEncoding.DecodeString(data); err == nil {
				_ = json.Unmarshal(decoded, &seen.payload)
			}
		}
		w.WriteHeader(status)
		out, _ := json.Marshal(body)
		_, _ = w.Write(out)
	}))
	t.Cleanup(server.Close)
	return server, seen
}

func clientFor(server *httptest.Server) *Client {
	return New(testPublicKey, testMerchantPK, WithBaseURL(server.URL), WithLanguage("az"))
}

func TestCreatePaymentSignsAndParses(t *testing.T) {
	server, seen := recording(t, 200, map[string]any{"status": "success", "transaction": "te1"})
	client := clientFor(server)

	result, err := client.CreatePayment(context.Background(), 30.75, "order-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK() || result.Transaction() != "te1" {
		t.Fatalf("unexpected result: %+v", result.Raw)
	}
	if seen.payload["public_key"] != testPublicKey {
		t.Fatalf("public_key not sent: %v", seen.payload["public_key"])
	}
	if seen.payload["amount"] != "30.75" {
		t.Fatalf("amount not formatted: %v", seen.payload["amount"])
	}
	if !signatureMatches(testMerchantPK, seen.form["data"], seen.form["signature"]) {
		t.Fatal("signature did not verify")
	}
}

func TestAmountFormatting(t *testing.T) {
	server, seen := recording(t, 200, map[string]any{"status": "success"})
	_, err := clientFor(server).CreatePayment(context.Background(), 5, "o", nil)
	if err != nil {
		t.Fatal(err)
	}
	if seen.payload["amount"] != "5.00" {
		t.Fatalf("expected 5.00, got %v", seen.payload["amount"])
	}
}

func TestEmptyFieldsAreDropped(t *testing.T) {
	server, seen := recording(t, 200, map[string]any{"status": "success"})
	_, err := clientFor(server).CreatePayment(context.Background(), 1, "o", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := seen.payload["description"]; ok {
		t.Fatal("empty description should have been dropped")
	}
	if _, ok := seen.payload["other_attr"]; ok {
		t.Fatal("nil other_attr should have been dropped")
	}
}

func TestLanguageDefaultAndOverride(t *testing.T) {
	server, seen := recording(t, 200, map[string]any{"status": "success"})
	client := clientFor(server)

	_, _ = client.CreatePayment(context.Background(), 1, "a", nil)
	if seen.payload["language"] != "az" {
		t.Fatalf("expected az, got %v", seen.payload["language"])
	}
	_, _ = client.CreatePayment(context.Background(), 1, "b", &PaymentOptions{Language: "en"})
	if seen.payload["language"] != "en" {
		t.Fatalf("expected en, got %v", seen.payload["language"])
	}
}

func TestErrorStatusReturnsGatewayError(t *testing.T) {
	server, _ := recording(t, 200, map[string]any{"status": "error", "code": "116", "message": "no funds"})
	_, err := clientFor(server).GetStatus(context.Background(), "te1")

	var gwErr *GatewayError
	if !errors.As(err, &gwErr) {
		t.Fatalf("expected GatewayError, got %v", err)
	}
	if gwErr.Code != "116" {
		t.Fatalf("expected code 116, got %s", gwErr.Code)
	}
}

func TestFailedStatusIsReturnedNotErrored(t *testing.T) {
	server, _ := recording(t, 200, map[string]any{"status": "failed", "code": "116"})
	result, err := clientFor(server).GetStatus(context.Background(), "te1")
	if err != nil {
		t.Fatalf("failed status should not be an error: %v", err)
	}
	if result.OK() || result.Status() != "failed" {
		t.Fatalf("unexpected result: %+v", result.Raw)
	}
	if result.String("code") != "116" {
		t.Fatalf("expected code 116, got %s", result.String("code"))
	}
}

func TestNonJSONBodyIsTransportError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>down</html>"))
	}))
	t.Cleanup(server.Close)

	_, err := clientFor(server).GetStatus(context.Background(), "te1")
	var transErr *TransportError
	if !errors.As(err, &transErr) {
		t.Fatalf("expected TransportError, got %v", err)
	}
}

func TestHTTP500IsTransportError(t *testing.T) {
	server, _ := recording(t, 500, map[string]any{"detail": "boom"})
	_, err := clientFor(server).GetStatus(context.Background(), "te1")

	var transErr *TransportError
	if !errors.As(err, &transErr) {
		t.Fatalf("expected TransportError, got %v", err)
	}
	if transErr.StatusCode != 500 {
		t.Fatalf("expected status 500, got %d", transErr.StatusCode)
	}
}

func TestBankTransferSendsJSON(t *testing.T) {
	server, seen := recording(t, 200, map[string]any{"status": "success", "order_id": "b1"})
	_, err := clientFor(server).CreateBankTransfer(context.Background(), BankTransfer{
		OrderID:     "b1",
		Amount:      100,
		Description: "fees",
		IBAN:        "AZ21NABZ00000000137010001944",
		Name:        "ACME",
		TIN:         "1234567892",
		BankCode:    "505141",
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen.contentType != "application/json" {
		t.Fatalf("expected JSON, got %s", seen.contentType)
	}
	if _, ok := seen.jsonBody["data"]; !ok {
		t.Fatal("data missing from JSON body")
	}
	if _, ok := seen.jsonBody["signature"]; !ok {
		t.Fatal("signature missing from JSON body")
	}
}

func TestRegisterCardSendsTheRedirectURLsItIsGiven(t *testing.T) {
	server, seen := recording(t, 200, map[string]any{"status": "success", "card_id": "ce1"})
	_, err := clientFor(server).RegisterCard(context.Background(), &RegisterCardOptions{
		SuccessRedirectURL: "https://shop.test/ok",
		ErrorRedirectURL:   "https://shop.test/fail",
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen.payload["success_redirect_url"] != "https://shop.test/ok" {
		t.Fatalf("success_redirect_url = %v", seen.payload["success_redirect_url"])
	}
	if seen.payload["error_redirect_url"] != "https://shop.test/fail" {
		t.Fatalf("error_redirect_url = %v", seen.payload["error_redirect_url"])
	}
}

func TestRegisterCardFallsBackToTheClientRedirectURLs(t *testing.T) {
	server, seen := recording(t, 200, map[string]any{"status": "success", "card_id": "ce1"})
	client := New(testPublicKey, testMerchantPK, WithBaseURL(server.URL),
		WithRedirectURLs("https://shop.test/default-ok", "https://shop.test/default-fail"))
	ctx := context.Background()

	if _, err := client.RegisterCard(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if seen.payload["success_redirect_url"] != "https://shop.test/default-ok" ||
		seen.payload["error_redirect_url"] != "https://shop.test/default-fail" {
		t.Fatalf("defaults not used: %v", seen.payload)
	}

	if _, err := client.RegisterCard(ctx, &RegisterCardOptions{SuccessRedirectURL: "https://shop.test/ok"}); err != nil {
		t.Fatal(err)
	}
	if seen.payload["success_redirect_url"] != "https://shop.test/ok" ||
		seen.payload["error_redirect_url"] != "https://shop.test/default-fail" {
		t.Fatalf("override not used: %v", seen.payload)
	}

	bare, bareSeen := recording(t, 200, map[string]any{"status": "success", "card_id": "ce1"})
	if _, err := clientFor(bare).RegisterCard(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := bareSeen.payload["success_redirect_url"]; ok {
		t.Fatalf("empty success_redirect_url was sent: %v", bareSeen.payload)
	}
	if _, ok := bareSeen.payload["error_redirect_url"]; ok {
		t.Fatalf("empty error_redirect_url was sent: %v", bareSeen.payload)
	}
}
