package epoint

import (
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
)

func encodePayload(payload map[string]any) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(body), nil
}

func sign(privateKey, data string) string {
	digest := sha1.Sum([]byte(privateKey + data + privateKey))
	return base64.StdEncoding.EncodeToString(digest[:])
}

func signatureMatches(privateKey, data, signature string) bool {
	expected := sign(privateKey, data)
	return subtle.ConstantTimeCompare([]byte(expected), []byte(signature)) == 1
}

func decodeData(data string) (map[string]any, error) {
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, &SignatureError{Message: "data is not valid base64"}
	}
	var payload map[string]any
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return nil, &SignatureError{Message: "data did not decode to an object"}
	}
	return payload, nil
}
