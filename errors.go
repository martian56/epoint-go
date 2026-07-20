package epoint

import "fmt"

type GatewayError struct {
	Message string
	Code    string
	Status  string
	Payload map[string]any
}

func (e *GatewayError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("epoint gateway error %s: %s", e.Code, e.Message)
	}
	return "epoint gateway error: " + e.Message
}

type TransportError struct {
	Message    string
	StatusCode int
}

func (e *TransportError) Error() string {
	return "epoint transport error: " + e.Message
}

type SignatureError struct {
	Message string
}

func (e *SignatureError) Error() string {
	return "epoint signature error: " + e.Message
}
