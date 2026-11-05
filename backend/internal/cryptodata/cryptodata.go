// Package cryptodata encrypts tenant data using versioned tenant data keys.
// Key persistence and master-key custody are handled by the caller.
package cryptodata

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
)

const KeySize = 32

func NewDataKey() ([]byte, error) {
	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}

// WrapKey uses a server-held master key to protect one tenant data key.
// The result is nonce || ciphertext, including the GCM authentication tag.
func WrapKey(master, dataKey []byte, masterID, tenantID string, version uint32) ([]byte, error) {
	if len(dataKey) != KeySize || masterID == "" || tenantID == "" || version == 0 {
		return nil, errors.New("invalid key metadata")
	}
	return seal(master, dataKey, associatedData("tenant-key", masterID, tenantID, version))
}

func UnwrapKey(master, wrapped []byte, masterID, tenantID string, version uint32) ([]byte, error) {
	if masterID == "" || tenantID == "" || version == 0 {
		return nil, errors.New("invalid key metadata")
	}
	key, err := open(master, wrapped, associatedData("tenant-key", masterID, tenantID, version))
	if err != nil {
		return nil, err
	}
	if len(key) != KeySize {
		return nil, errors.New("invalid unwrapped data key")
	}
	return key, nil
}

func WrapSearchKey(master, searchKey []byte, masterID, tenantID string) ([]byte, error) {
	if len(searchKey) != KeySize || masterID == "" || tenantID == "" {
		return nil, errors.New("invalid search-key metadata")
	}
	return seal(master, searchKey, associatedData("tenant-search-key", masterID, tenantID))
}

func UnwrapSearchKey(master, wrapped []byte, masterID, tenantID string) ([]byte, error) {
	if masterID == "" || tenantID == "" {
		return nil, errors.New("invalid search-key metadata")
	}
	key, err := open(master, wrapped, associatedData("tenant-search-key", masterID, tenantID))
	if err != nil {
		return nil, err
	}
	if len(key) != KeySize {
		return nil, errors.New("invalid unwrapped search key")
	}
	return key, nil
}

// EncryptField binds ciphertext to its tenant, record, field, and key version.
// Moving a ciphertext to another tenant or column therefore fails authentication.
func EncryptField(dataKey []byte, tenantID, recordID, field string, version uint32, plaintext []byte) ([]byte, error) {
	if tenantID == "" || recordID == "" || field == "" || version == 0 {
		return nil, errors.New("invalid field metadata")
	}
	return seal(dataKey, plaintext, associatedData("field", tenantID, recordID, field, version))
}

func DecryptField(dataKey, ciphertext []byte, tenantID, recordID, field string, version uint32) ([]byte, error) {
	if tenantID == "" || recordID == "" || field == "" || version == 0 {
		return nil, errors.New("invalid field metadata")
	}
	return open(dataKey, ciphertext, associatedData("field", tenantID, recordID, field, version))
}

// BlindIndex accepts an independently stored, stable 32-byte tenant search
// key. Rotating the data-encryption key does not change searchable hashes.
func BlindIndex(searchKey []byte, tenantID, normalizedValue string) ([32]byte, error) {
	var result [32]byte
	if len(searchKey) != KeySize || tenantID == "" || normalizedValue == "" {
		return result, errors.New("invalid blind-index input")
	}
	h := hmac.New(sha256.New, searchKey)
	h.Write(associatedData("index", tenantID, normalizedValue))
	copy(result[:], h.Sum(nil))
	return result, nil
}

func seal(key, plaintext, aad []byte) ([]byte, error) {
	if len(key) != KeySize {
		return nil, errors.New("AES-256 key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, aad), nil
}

func open(key, ciphertext, aad []byte) ([]byte, error) {
	if len(key) != KeySize {
		return nil, errors.New("AES-256 key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize()+gcm.Overhead() {
		return nil, errors.New("ciphertext is too short")
	}
	return gcm.Open(nil, ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():], aad)
}

// Length-prefix each component to prevent ambiguous concatenations.
func associatedData(parts ...any) []byte {
	var result []byte
	for _, part := range parts {
		value := []byte(fmt.Sprint(part))
		var size [4]byte
		binary.BigEndian.PutUint32(size[:], uint32(len(value)))
		result = append(result, size[:]...)
		result = append(result, value...)
	}
	return result
}
