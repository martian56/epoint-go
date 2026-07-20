package epoint

import (
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"testing"
)

const testPrivateKey = "d3hjsl38sd8kdfhbcea0be04eafde9e8e2bad2fb092d"

func reference(privateKey, data string) string {
	digest := sha1.Sum([]byte(privateKey + data + privateKey))
	return base64.StdEncoding.EncodeToString(digest[:])
}

func TestSignMatchesDocumentedFormula(t *testing.T) {
	data, err := encodePayload(map[string]any{"public_key": "i000000001", "amount": "30.75"})
	if err != nil {
		t.Fatal(err)
	}
	if got := sign(testPrivateKey, data); got != reference(testPrivateKey, data) {
		t.Fatalf("signature mismatch: %s", got)
	}
}

func TestSignUsesRawDigestNotHex(t *testing.T) {
	data, _ := encodePayload(map[string]any{"amount": "1.00"})
	digest := sha1.Sum([]byte(testPrivateKey + data + testPrivateKey))
	hexEncoded := base64.StdEncoding.EncodeToString([]byte(hexString(digest[:])))
	if sign(testPrivateKey, data) == hexEncoded {
		t.Fatal("signature used the hex digest instead of raw bytes")
	}
}

func hexString(b []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = digits[v>>4]
		out[i*2+1] = digits[v&0x0f]
	}
	return string(out)
}

func TestSignatureMatches(t *testing.T) {
	data, _ := encodePayload(map[string]any{"amount": "5.00"})
	if !signatureMatches(testPrivateKey, data, sign(testPrivateKey, data)) {
		t.Fatal("correct signature was rejected")
	}
	if signatureMatches(testPrivateKey, data, "wrong") {
		t.Fatal("wrong signature was accepted")
	}
}

func TestDecodeDataRejectsGarbage(t *testing.T) {
	_, err := decodeData("not base64")
	var sigErr *SignatureError
	if !errors.As(err, &sigErr) {
		t.Fatalf("expected SignatureError, got %v", err)
	}
}

func TestVerifyCallback(t *testing.T) {
	payload := map[string]any{
		"order_id":    "order-1",
		"status":      "success",
		"code":        "000",
		"transaction": "te0000000001",
		"amount":      30.75,
	}
	data, _ := encodePayload(payload)
	callback, err := VerifyCallback(testPrivateKey, data, sign(testPrivateKey, data))
	if err != nil {
		t.Fatal(err)
	}
	if !callback.OK() {
		t.Fatal("callback should be ok")
	}
	if callback.OrderID != "order-1" || callback.Transaction != "te0000000001" {
		t.Fatalf("unexpected callback: %+v", callback)
	}
	if callback.Amount != 30.75 {
		t.Fatalf("unexpected amount: %v", callback.Amount)
	}
}

func TestVerifyCallbackRejectsTampering(t *testing.T) {
	data, _ := encodePayload(map[string]any{"order_id": "order-1", "amount": 1})
	good := sign(testPrivateKey, data)
	tampered, _ := encodePayload(map[string]any{"order_id": "order-1", "amount": 9999})

	_, err := VerifyCallback(testPrivateKey, tampered, good)
	var sigErr *SignatureError
	if !errors.As(err, &sigErr) {
		t.Fatalf("expected SignatureError, got %v", err)
	}
}
