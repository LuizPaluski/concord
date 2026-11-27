package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"time"
)

type livekit struct {
	URL, Key, Secret string
}

// token assina o JWT de acesso do LiveKit (HS256), o mesmo que o SDK oficial gera.
func (lk livekit) token(identity, room string) string {
	enc := base64.RawURLEncoding.EncodeToString
	now := time.Now()
	claims, _ := json.Marshal(map[string]any{
		"iss":  lk.Key,
		"sub":  identity,
		"name": identity,
		"nbf":  now.Unix(),
		"exp":  now.Add(6 * time.Hour).Unix(),
		"video": map[string]any{
			"room":         room,
			"roomJoin":     true,
			"canPublish":   true,
			"canSubscribe": true,
		},
	})
	unsigned := enc([]byte(`{"alg":"HS256","typ":"JWT"}`)) + "." + enc(claims)
	mac := hmac.New(sha256.New, []byte(lk.Secret))
	mac.Write([]byte(unsigned))
	return unsigned + "." + enc(mac.Sum(nil))
}
