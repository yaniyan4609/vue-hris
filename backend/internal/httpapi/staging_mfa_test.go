package httpapi

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func completeStagingMFA(t *testing.T, client *http.Client, base string) string {
	t.Helper()
	var status struct {
		Data struct {
			Required bool `json:"required"`
			Enrolled bool `json:"enrolled"`
		} `json:"data"`
	}
	doJSON(t, client, "GET", base+"/api/v1/auth/mfa/status", nil, 200, &status)
	if !status.Data.Required || status.Data.Enrolled {
		t.Fatalf("unexpected MFA status: %+v", status)
	}
	var setup struct {
		Data struct {
			Secret string `json:"secret"`
		} `json:"data"`
	}
	doJSON(t, client, "POST", base+"/api/v1/auth/mfa/setup", map[string]any{}, 200, &setup)
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(setup.Data.Secret)
	if err != nil {
		t.Fatal(err)
	}
	doJSON(t, client, "POST", base+"/api/v1/auth/mfa/confirm", map[string]string{"code": "invalid"}, 403, nil)
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], uint64(time.Now().Unix()/30))
	mac := hmac.New(sha1.New, secret)
	mac.Write(counter[:])
	digest := mac.Sum(nil)
	offset := digest[len(digest)-1] & 15
	code := fmt.Sprintf("%06d", (binary.BigEndian.Uint32(digest[offset:offset+4])&0x7fffffff)%1_000_000)
	var confirmed struct {
		Data struct {
			RecoveryCodes []string `json:"recovery_codes"`
		} `json:"data"`
	}
	doJSON(t, client, "POST", base+"/api/v1/auth/mfa/confirm", map[string]string{"code": code}, 200, &confirmed)
	if len(confirmed.Data.RecoveryCodes) != 10 {
		t.Fatal("recovery codes missing")
	}
	return confirmed.Data.RecoveryCodes[0]
}
