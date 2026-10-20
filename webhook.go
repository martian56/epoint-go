package epoint

import (
	"encoding/json"
	"fmt"
	"strings"
)

type B2BWebhook struct {
	OrderID      string
	Status       string
	Amount       float64
	PayeeIBAN    string
	PayeeName    string
	ErrorMessage string
	Raw          map[string]any
}

func (w *B2BWebhook) OK() bool {
	return strings.EqualFold(w.Status, "success")
}

func ParseB2BWebhook(body []byte) (*B2BWebhook, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil || raw == nil {
		return nil, fmt.Errorf("epoint b2b webhook: body is not a JSON object")
	}
	return &B2BWebhook{
		OrderID:      text(raw["order_id"]),
		Status:       text(raw["status"]),
		Amount:       number(raw["amount"]),
		PayeeIBAN:    text(raw["payee_iban"]),
		PayeeName:    text(raw["payee_name"]),
		ErrorMessage: text(raw["error_message"]),
		Raw:          raw,
	}, nil
}
