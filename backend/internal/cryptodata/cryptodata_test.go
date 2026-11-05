package cryptodata

import (
	"bytes"
	"testing"
)

func TestTenantAndFieldBinding(t *testing.T) {
	master, err := NewDataKey()
	if err != nil {
		t.Fatal(err)
	}
	dataKey, err := NewDataKey()
	if err != nil {
		t.Fatal(err)
	}
	wrapped, err := WrapKey(master, dataKey, "master-1", "tenant-a", 1)
	if err != nil {
		t.Fatal(err)
	}
	unwrapped, err := UnwrapKey(master, wrapped, "master-1", "tenant-a", 1)
	if err != nil || !bytes.Equal(unwrapped, dataKey) {
		t.Fatal("tenant key could not be unwrapped")
	}
	if _, err := UnwrapKey(master, wrapped, "master-1", "tenant-b", 1); err == nil {
		t.Fatal("wrapped key was accepted for another tenant")
	}
	ciphertext, err := EncryptField(dataKey, "tenant-a", "employee-a", "nik", 1, []byte("sensitive-test-value"))
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := DecryptField(dataKey, ciphertext, "tenant-a", "employee-a", "nik", 1)
	if err != nil || string(plaintext) != "sensitive-test-value" {
		t.Fatal("field could not be decrypted")
	}
	if _, err := DecryptField(dataKey, ciphertext, "tenant-b", "employee-a", "nik", 1); err == nil {
		t.Fatal("ciphertext was accepted for another tenant")
	}
	if _, err := DecryptField(dataKey, ciphertext, "tenant-a", "employee-a", "npwp", 1); err == nil {
		t.Fatal("ciphertext was accepted for another field")
	}
}

func TestSearchKeyIsSeparateFromDataKey(t *testing.T) {
	master, _ := NewDataKey()
	searchKey, _ := NewDataKey()
	wrapped, err := WrapSearchKey(master, searchKey, "master-1", "tenant-a")
	if err != nil {
		t.Fatal(err)
	}
	got, err := UnwrapSearchKey(master, wrapped, "master-1", "tenant-a")
	if err != nil || !bytes.Equal(got, searchKey) {
		t.Fatal("search key could not be unwrapped")
	}
	if _, err := UnwrapKey(master, wrapped, "master-1", "tenant-a", 1); err == nil {
		t.Fatal("search key was accepted as a data key")
	}
	first, err := BlindIndex(searchKey, "tenant-a", "1234567890")
	if err != nil {
		t.Fatal(err)
	}
	otherTenant, _ := BlindIndex(searchKey, "tenant-b", "1234567890")
	if first == otherTenant {
		t.Fatal("blind index was not bound to tenant")
	}
}
