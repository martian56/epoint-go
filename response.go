package epoint

import "fmt"

type Response struct {
	Raw map[string]any
}

func (r *Response) Status() string {
	return r.String("status")
}

func (r *Response) OK() bool {
	return Status(r.String("status")).Settled()
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

func (r *Response) Get(key string) any {
	return r.Raw[key]
}

func (r *Response) String(key string) string {
	value, ok := r.Raw[key]
	if !ok || value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return fmt.Sprint(value)
}

type Callback struct {
	OrderID         string
	Status          string
	Code            string
	Message         string
	Transaction     string
	BankTransaction string
	OperationCode   string
	RRN             string
	CardMask        string
	CardName        string
	Amount          float64
	OtherAttr       any
	Raw             map[string]any
}

func (c *Callback) OK() bool {
	return Status(c.Status).Settled()
}

func toCallback(raw map[string]any) *Callback {
	str := func(key string) string {
		if v, ok := raw[key]; ok && v != nil {
			if s, ok := v.(string); ok {
				return s
			}
			return fmt.Sprint(v)
		}
		return ""
	}
	amount := 0.0
	if v, ok := raw["amount"].(float64); ok {
		amount = v
	}
	return &Callback{
		OrderID:         str("order_id"),
		Status:          str("status"),
		Code:            str("code"),
		Message:         str("message"),
		Transaction:     str("transaction"),
		BankTransaction: str("bank_transaction"),
		OperationCode:   str("operation_code"),
		RRN:             str("rrn"),
		CardMask:        str("card_mask"),
		CardName:        str("card_name"),
		Amount:          amount,
		OtherAttr:       raw["other_attr"],
		Raw:             raw,
	}
}
