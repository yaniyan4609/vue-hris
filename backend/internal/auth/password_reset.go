package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"time"
)

var ErrResetTokenInvalid = errors.New("tautan reset tidak sah atau kedaluwarsa")

// IssuePasswordReset deliberately returns no account-existence signal. The
// caller must deliver the token out of band and never include it in an API body.
func (s Service) IssuePasswordReset(ctx context.Context, email string, deliver func(string, string) error) error {
	var userID, destination string
	err := s.DB.QueryRowContext(ctx, `SELECT id,email FROM users WHERE email_normalized=? AND status='active'`, strings.ToLower(strings.TrimSpace(email))).Scan(&userID, &destination)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var recent int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM password_reset_tokens WHERE user_id=? AND created_at>UTC_TIMESTAMP(6)-INTERVAL 1 MINUTE`, userID).Scan(&recent); err != nil {
		return err
	}
	if recent > 0 {
		return nil
	}
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	hash := sha256.Sum256(raw[:])
	if _, err := s.DB.ExecContext(ctx, `INSERT INTO password_reset_tokens (token_hash,user_id,expires_at) VALUES (?,?,UTC_TIMESTAMP(6)+INTERVAL 15 MINUTE)`, hash[:], userID); err != nil {
		return err
	}
	if err := deliver(destination, token); err != nil {
		_, _ = s.DB.ExecContext(ctx, `DELETE FROM password_reset_tokens WHERE token_hash=?`, hash[:])
		return err
	}
	return nil
}

func (s Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	if len([]rune(newPassword)) < 8 || len(newPassword) > 1024 {
		return ErrPasswordPolicy
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 32 {
		return ErrResetTokenInvalid
	}
	hash := sha256.Sum256(raw)
	newHash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var userID string
	var expires time.Time
	var used sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT user_id,expires_at,used_at FROM password_reset_tokens WHERE token_hash=? FOR UPDATE`, hash[:]).Scan(&userID, &expires, &used)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrResetTokenInvalid
	}
	if err != nil {
		return err
	}
	if used.Valid || !time.Now().UTC().Before(expires) {
		return ErrResetTokenInvalid
	}
	result, err := tx.ExecContext(ctx, `UPDATE users SET password_hash=? WHERE id=? AND status='active'`, newHash, userID)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return ErrResetTokenInvalid
	}
	if _, err := tx.ExecContext(ctx, `UPDATE password_reset_tokens SET used_at=UTC_TIMESTAMP(6) WHERE user_id=? AND used_at IS NULL`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE sessions SET revoked_at=UTC_TIMESTAMP(6),revoked_reason='password_reset',reauthenticated_at=NULL WHERE user_id=? AND revoked_at IS NULL`, userID); err != nil {
		return err
	}
	return tx.Commit()
}
