package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

var (
	ErrMFASetupRequired = errors.New("authenticator belum diaktifkan")
	ErrMFAEnrolled      = errors.New("authenticator sudah diaktifkan")
	ErrMFAInvalid       = errors.New("kode authenticator tidak sah")
)

func platformMFAKey() []byte {
	key, err := base64.StdEncoding.DecodeString(os.Getenv("PLATFORM_MFA_KEY"))
	if err != nil || len(key) != 32 {
		return nil
	}
	return key
}

func sealMFASecret(secret []byte) ([]byte, []byte, error) {
	key := platformMFAKey()
	if key == nil {
		return nil, nil, ErrMFAUnavailable
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	return nonce, gcm.Seal(nil, nonce, secret, nil), nil
}

func openMFASecret(nonce, ciphertext []byte) ([]byte, error) {
	key := platformMFAKey()
	if key == nil {
		return nil, ErrMFAUnavailable
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func totp(secret []byte, counter uint64) string {
	var value [8]byte
	binary.BigEndian.PutUint64(value[:], counter)
	mac := hmac.New(sha1.New, secret)
	mac.Write(value[:])
	digest := mac.Sum(nil)
	offset := digest[len(digest)-1] & 0x0f
	number := binary.BigEndian.Uint32(digest[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", number%1_000_000)
}

func matchingTOTPCounter(secret []byte, code string, now time.Time, last uint64) (uint64, bool) {
	if len(code) != 6 {
		return 0, false
	}
	for _, char := range code {
		if char < '0' || char > '9' {
			return 0, false
		}
	}
	current := now.Unix() / 30
	for offset := int64(-1); offset <= 1; offset++ {
		candidate := current + offset
		if candidate > 0 && uint64(candidate) > last && subtle.ConstantTimeCompare([]byte(totp(secret, uint64(candidate))), []byte(code)) == 1 {
			return uint64(candidate), true
		}
	}
	return 0, false
}

func (s Service) MFAStatus(ctx context.Context, token string) (bool, bool, error) {
	state, err := s.Pending(ctx, token)
	if err != nil {
		return false, false, err
	}
	if !state.Platform {
		return false, false, ErrTenantUnavailable
	}
	var confirmed sql.NullTime
	err = s.DB.QueryRowContext(ctx, `SELECT confirmed_at FROM platform_mfa_credentials WHERE user_id=?`, state.User.ID).Scan(&confirmed)
	if errors.Is(err, sql.ErrNoRows) {
		return state.MFARequired, false, nil
	}
	return state.MFARequired, confirmed.Valid, err
}

func (s Service) MFASetup(ctx context.Context, token string) (string, string, error) {
	state, err := s.Pending(ctx, token)
	if err != nil {
		return "", "", err
	}
	if !state.Platform {
		return "", "", ErrTenantUnavailable
	}
	var confirmed sql.NullTime
	err = s.DB.QueryRowContext(ctx, `SELECT confirmed_at FROM platform_mfa_credentials WHERE user_id=?`, state.User.ID).Scan(&confirmed)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", "", err
	}
	if confirmed.Valid {
		return "", "", ErrMFAEnrolled
	}
	secret := make([]byte, 20)
	if _, err := rand.Read(secret); err != nil {
		return "", "", err
	}
	nonce, ciphertext, err := sealMFASecret(secret)
	if err != nil {
		return "", "", err
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO platform_mfa_credentials (user_id,secret_nonce,secret_ciphertext) VALUES (?,?,?) ON DUPLICATE KEY UPDATE secret_nonce=VALUES(secret_nonce),secret_ciphertext=VALUES(secret_ciphertext),last_counter=0`, state.User.ID, nonce, ciphertext)
	if err != nil {
		return "", "", err
	}
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret)
	label := "digaji.id:" + state.User.Email
	uri := "otpauth://totp/" + url.PathEscape(label) + "?secret=" + encoded + "&issuer=digaji.id&algorithm=SHA1&digits=6&period=30"
	return encoded, uri, nil
}

func (s Service) MFAConfirm(ctx context.Context, token, code string) ([]string, State, error) {
	state, err := s.Pending(ctx, token)
	if err != nil {
		return nil, State{}, err
	}
	if !state.Platform {
		return nil, State{}, ErrTenantUnavailable
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, State{}, err
	}
	defer tx.Rollback()
	var nonce, ciphertext []byte
	var confirmed sql.NullTime
	if err := tx.QueryRowContext(ctx, `SELECT secret_nonce,secret_ciphertext,confirmed_at FROM platform_mfa_credentials WHERE user_id=? FOR UPDATE`, state.User.ID).Scan(&nonce, &ciphertext, &confirmed); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, State{}, ErrMFASetupRequired
		}
		return nil, State{}, err
	}
	if confirmed.Valid {
		return nil, State{}, ErrMFAEnrolled
	}
	secret, err := openMFASecret(nonce, ciphertext)
	if err != nil {
		return nil, State{}, err
	}
	counter, valid := matchingTOTPCounter(secret, code, time.Now().UTC(), 0)
	if !valid {
		hash, _ := tokenHash(token)
		if _, err := tx.ExecContext(ctx, `UPDATE sessions SET mfa_failures=mfa_failures+1,revoked_at=CASE WHEN mfa_failures>=4 THEN UTC_TIMESTAMP(6) ELSE revoked_at END,revoked_reason=CASE WHEN mfa_failures>=4 THEN 'mfa_failed' ELSE revoked_reason END WHERE token_hash=? AND revoked_at IS NULL`, hash[:]); err != nil {
			return nil, State{}, err
		}
		if err := tx.Commit(); err != nil {
			return nil, State{}, err
		}
		return nil, State{}, ErrMFAInvalid
	}
	if _, err := tx.ExecContext(ctx, `UPDATE platform_mfa_credentials SET confirmed_at=UTC_TIMESTAMP(6),last_counter=? WHERE user_id=?`, counter, state.User.ID); err != nil {
		return nil, State{}, err
	}
	codes := make([]string, 10)
	for i := range codes {
		raw := make([]byte, 10)
		if _, err := rand.Read(raw); err != nil {
			return nil, State{}, err
		}
		codes[i] = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
		hash := sha256.Sum256([]byte(codes[i]))
		if _, err := tx.ExecContext(ctx, `INSERT INTO platform_mfa_recovery_codes (user_id,code_hash) VALUES (?,?)`, state.User.ID, hash[:]); err != nil {
			return nil, State{}, err
		}
	}
	hash, _ := tokenHash(token)
	if _, err := tx.ExecContext(ctx, `UPDATE sessions SET mfa_verified_at=UTC_TIMESTAMP(6),mfa_failures=0 WHERE token_hash=? AND user_id=? AND revoked_at IS NULL`, hash[:], state.User.ID); err != nil {
		return nil, State{}, err
	}
	if err := tx.Commit(); err != nil {
		return nil, State{}, err
	}
	state.MFARequired = false
	return codes, state, nil
}

func (s Service) MFAVerify(ctx context.Context, token, code string) (State, error) {
	state, err := s.Pending(ctx, token)
	if err != nil {
		return State{}, err
	}
	if !state.Platform || !state.MFARequired {
		return State{}, ErrMFARequired
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return State{}, err
	}
	defer tx.Rollback()
	var nonce, ciphertext []byte
	var confirmed sql.NullTime
	var last uint64
	err = tx.QueryRowContext(ctx, `SELECT secret_nonce,secret_ciphertext,confirmed_at,last_counter FROM platform_mfa_credentials WHERE user_id=? FOR UPDATE`, state.User.ID).Scan(&nonce, &ciphertext, &confirmed, &last)
	if errors.Is(err, sql.ErrNoRows) || !confirmed.Valid && err == nil {
		return State{}, ErrMFASetupRequired
	}
	if err != nil {
		return State{}, err
	}
	secret, err := openMFASecret(nonce, ciphertext)
	if err != nil {
		return State{}, err
	}
	counter, valid := matchingTOTPCounter(secret, code, time.Now().UTC(), last)
	if valid {
		_, err = tx.ExecContext(ctx, `UPDATE platform_mfa_credentials SET last_counter=? WHERE user_id=?`, counter, state.User.ID)
		if err != nil {
			return State{}, err
		}
	} else if len(code) == 16 {
		recovery := sha256.Sum256([]byte(strings.ToUpper(code)))
		result, err := tx.ExecContext(ctx, `UPDATE platform_mfa_recovery_codes SET used_at=UTC_TIMESTAMP(6) WHERE user_id=? AND code_hash=? AND used_at IS NULL`, state.User.ID, recovery[:])
		if err != nil {
			return State{}, err
		}
		changed, err := result.RowsAffected()
		valid = err == nil && changed == 1
	}
	hash, _ := tokenHash(token)
	if !valid {
		if _, err := tx.ExecContext(ctx, `UPDATE sessions SET mfa_failures=mfa_failures+1,revoked_at=CASE WHEN mfa_failures>=4 THEN UTC_TIMESTAMP(6) ELSE revoked_at END,revoked_reason=CASE WHEN mfa_failures>=4 THEN 'mfa_failed' ELSE revoked_reason END WHERE token_hash=? AND revoked_at IS NULL`, hash[:]); err != nil {
			return State{}, err
		}
		if err := tx.Commit(); err != nil {
			return State{}, err
		}
		return State{}, ErrMFAInvalid
	}
	if _, err := tx.ExecContext(ctx, `UPDATE sessions SET mfa_verified_at=UTC_TIMESTAMP(6),mfa_failures=0 WHERE token_hash=? AND user_id=? AND revoked_at IS NULL`, hash[:], state.User.ID); err != nil {
		return State{}, err
	}
	if err := tx.Commit(); err != nil {
		return State{}, err
	}
	state.MFARequired = false
	return state, nil
}
