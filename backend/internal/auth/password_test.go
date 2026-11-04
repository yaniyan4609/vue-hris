package auth

import "testing"

func TestPasswordHash(t *testing.T) {
	hash, err := HashPassword("password")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "password" || !VerifyPassword(hash, "password") || VerifyPassword(hash, "wrong") {
		t.Fatal("password verification failed")
	}
	if VerifyPassword("invalid", "password") {
		t.Fatal("invalid hash accepted")
	}
}
