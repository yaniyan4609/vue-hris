package ids

import (
	"crypto/rand"
	"encoding/base32"
)

var encoding = base32.NewEncoding("0123456789ABCDEFGHJKMNPQRSTVWXYZ").WithPadding(base32.NoPadding)

// New returns a 26-character identifier backed by 128 bits of randomness.
func New() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return encoding.EncodeToString(raw[:]), nil
}
