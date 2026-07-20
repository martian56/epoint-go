package epoint

func VerifyCallback(privateKey, data, signature string) (*Callback, error) {
	if !signatureMatches(privateKey, data, signature) {
		return nil, &SignatureError{Message: "callback signature does not match"}
	}
	payload, err := decodeData(data)
	if err != nil {
		return nil, err
	}
	return toCallback(payload), nil
}
