package auth

import (
	"testing"
	"time"
)

func TestTOTPRFC6238SHA1(t *testing.T) {
	secret := []byte("12345678901234567890")
	for _, sample := range []struct {
		timestamp int64
		expected  string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
	} {
		got := totp(secret, uint64(sample.timestamp/30))
		if got != sample.expected {
			t.Fatalf("time %d: %s != %s", sample.timestamp, got, sample.expected)
		}
		counter, ok := matchingTOTPCounter(secret, got, time.Unix(sample.timestamp, 0), 0)
		if !ok || counter != uint64(sample.timestamp/30) {
			t.Fatalf("time %d did not verify", sample.timestamp)
		}
		if _, ok := matchingTOTPCounter(secret, got, time.Unix(sample.timestamp, 0), counter); ok {
			t.Fatalf("time %d replay accepted", sample.timestamp)
		}
	}
}
