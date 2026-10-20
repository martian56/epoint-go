package epoint

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type Response struct {
	Raw map[string]any
}

func (r *Response) Status() string {
	return r.String("status")
}

func (r *Response) OK() bool {
	switch r.String("status") {
	case string(StatusSuccess), string(B2BStatusSuccess), "ok":
		return true
	}
	return false
}

func (r *Response) Transaction() string {
	return r.String("transaction")
}

func (r *Response) RedirectURL() string {
	return r.String("redirect_url")
}

func (r *Response) Message() string {
	return r.String("message")
}

func (r *Response) TraceID() string {
	return r.String("trace_id")
}

func (r *Response) Code() string {
	return r.String("code")
}

func (r *Response) BankTransaction() string {
	return r.String("bank_transaction")
}

func (r *Response) BankResponse() string {
	return r.String("bank_response")
}

func (r *Response) OperationCode() string {
	return r.String("operation_code")
}

func (r *Response) RRN() string {
	return r.String("rrn")
}

func (r *Response) CardName() string {
	return r.String("card_name")
}

func (r *Response) CardMask() string {
	return r.String("card_mask")
}

func (r *Response) Amount() float64 {
	return number(r.Raw["amount"])
}

func (r *Response) CardID() string {
	return r.String("card_id")
}

func (r *Response) OrderID() string {
	return r.String("order_id")
}

func (r *Response) WidgetURL() string {
	return r.String("widget_url")
}

func (r *Response) Get(key string) any {
	return r.Raw[key]
}

func (r *Response) String(key string) string {
	return text(r.Raw[key])
}

func text(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

func number(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case json.Number:
		f, _ := v.Float64()
		return f
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return 0
		}
		return f
	}
	return 0
}

type Callback struct {
	OrderID         string
	Status          string
	Code            string
	Message         string
	Transaction     string
	BankTransaction string
	BankResponse    string
	OperationCode   string
	RRN             string
	CardMask        string
	CardName        string
	CardID          string
	TraceID         string
	Amount          float64
	OtherAttr       any
	Raw             map[string]any
}

func (c *Callback) OK() bool {
	return Status(c.Status).Settled()
}

func toCallback(raw map[string]any) *Callback {
	str := func(key string) string { return text(raw[key]) }
	return &Callback{
		OrderID:         str("order_id"),
		Status:          str("status"),
		Code:            str("code"),
		Message:         str("message"),
		Transaction:     str("transaction"),
		BankTransaction: str("bank_transaction"),
		BankResponse:    str("bank_response"),
		OperationCode:   str("operation_code"),
		RRN:             str("rrn"),
		CardMask:        str("card_mask"),
		CardName:        str("card_name"),
		CardID:          str("card_id"),
		TraceID:         str("trace_id"),
		Amount:          number(raw["amount"]),
		OtherAttr:       raw["other_attr"],
		Raw:             raw,
	}
}
