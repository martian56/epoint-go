package epoint

import "testing"

func TestOKForSuccessStates(t *testing.T) {
	for _, status := range []string{"success", "SUCCESS", "ok"} {
		if !(&Response{Raw: map[string]any{"status": status}}).OK() {
			t.Fatalf("%s should be ok", status)
		}
	}
	for _, status := range []string{"error", "failed", "server_error", "returned", "new", "FAILED", "PENDING", ""} {
		if (&Response{Raw: map[string]any{"status": status}}).OK() {
			t.Fatalf("%s should not be ok", status)
		}
	}
	if (&Response{Raw: map[string]any{}}).OK() {
		t.Fatal("a missing status should not be ok")
	}
}

func TestTypedResponseFields(t *testing.T) {
	r := &Response{Raw: map[string]any{
		"trace_id":         "tr1",
		"code":             "000",
		"bank_transaction": "BT1",
		"bank_response":    "001",
		"operation_code":   "100",
		"rrn":              "585858",
		"card_name":        "TESTER",
		"card_mask":        "411111******1111",
		"amount":           10.5,
		"card_id":          "ce1",
		"order_id":         "o1",
		"widget_url":       "https://w.test",
	}}
	checks := []struct{ got, want string }{
		{r.TraceID(), "tr1"},
		{r.Code(), "000"},
		{r.BankTransaction(), "BT1"},
		{r.BankResponse(), "001"},
		{r.OperationCode(), "100"},
		{r.RRN(), "585858"},
		{r.CardName(), "TESTER"},
		{r.CardMask(), "411111******1111"},
		{r.CardID(), "ce1"},
		{r.OrderID(), "o1"},
		{r.WidgetURL(), "https://w.test"},
	}
	for _, check := range checks {
		if check.got != check.want {
			t.Fatalf("got %q, want %q", check.got, check.want)
		}
	}
	if r.Amount() != 10.5 {
		t.Fatalf("amount = %v", r.Amount())
	}
	if r.Get("rrn") != "585858" || r.String("card_id") != "ce1" {
		t.Fatal("raw access changed")
	}
}

func TestResponseAmountFromStringAndMissing(t *testing.T) {
	if got := (&Response{Raw: map[string]any{"amount": "30.75"}}).Amount(); got != 30.75 {
		t.Fatalf("amount = %v", got)
	}
	if got := (&Response{Raw: map[string]any{"amount": "abc"}}).Amount(); got != 0 {
		t.Fatalf("amount = %v", got)
	}
	empty := &Response{Raw: map[string]any{"rrn": nil}}
	if empty.Amount() != 0 || empty.RRN() != "" || empty.TraceID() != "" {
		t.Fatalf("missing fields should be zero: %+v", empty.Raw)
	}
}

func TestCallbackNewFieldsAndStringAmount(t *testing.T) {
	data, _ := encodePayload(map[string]any{
		"order_id":      "o1",
		"status":        "success",
		"amount":        "25.50",
		"bank_response": "000",
		"trace_id":      "tr1",
		"card_id":       "ce1",
	})
	callback, err := VerifyCallback(testMerchantPK, data, sign(testMerchantPK, data))
	if err != nil {
		t.Fatal(err)
	}
	if callback.Amount != 25.5 || callback.BankResponse != "000" || callback.TraceID != "tr1" || callback.CardID != "ce1" {
		t.Fatalf("unexpected callback: %+v", callback)
	}
}

func TestCallbackNumericAmountAndMissingFields(t *testing.T) {
	data, _ := encodePayload(map[string]any{"order_id": "o1", "status": "failed", "amount": 12.0})
	callback, err := VerifyCallback(testMerchantPK, data, sign(testMerchantPK, data))
	if err != nil {
		t.Fatal(err)
	}
	if callback.Amount != 12 || callback.BankResponse != "" || callback.TraceID != "" || callback.CardID != "" {
		t.Fatalf("unexpected callback: %+v", callback)
	}
}

func TestParseB2BWebhook(t *testing.T) {
	hook, err := ParseB2BWebhook([]byte(`{"order_id":"b1","status":"success","amount":"1250.00","payee_iban":"AZ21NABZ00000000137010001944","payee_name":"Logistika MMC","error_message":null,"extra":"ignored"}`))
	if err != nil {
		t.Fatal(err)
	}
	if hook.OrderID != "b1" || hook.Status != "success" || hook.Amount != 1250 ||
		hook.PayeeIBAN != "AZ21NABZ00000000137010001944" || hook.PayeeName != "Logistika MMC" ||
		hook.ErrorMessage != "" || !hook.OK() {
		t.Fatalf("unexpected webhook: %+v", hook)
	}
}

func TestParseB2BWebhookFailedAndUnknownStatus(t *testing.T) {
	failed, err := ParseB2BWebhook([]byte(`{"order_id":"b2","status":"failed","amount":10,"error_message":"Invalid IBAN"}`))
	if err != nil {
		t.Fatal(err)
	}
	if failed.OK() || failed.Amount != 10 || failed.ErrorMessage != "Invalid IBAN" {
		t.Fatalf("unexpected webhook: %+v", failed)
	}
	odd, err := ParseB2BWebhook([]byte(`{"order_id":"b3","status":"on_hold"}`))
	if err != nil {
		t.Fatal(err)
	}
	if odd.Status != "on_hold" || odd.OK() {
		t.Fatalf("unexpected webhook: %+v", odd)
	}
}

func TestParseB2BWebhookRejectsNonObjects(t *testing.T) {
	for _, body := range []string{"", "not json", "[1,2]", "null", `"text"`} {
		if _, err := ParseB2BWebhook([]byte(body)); err == nil {
			t.Fatalf("expected an error for %q", body)
		}
	}
}
