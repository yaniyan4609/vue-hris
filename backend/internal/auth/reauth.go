package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

const ReauthDuration = 5 * time.Minute

func (s Service) Reauthenticate(ctx context.Context, token, password string) (time.Time, error) {
	state, err := s.Current(ctx, token)
	if err != nil {
		return time.Time{}, err
	}
	if password == "" || len(password) > 1024 {
		return time.Time{}, ErrInvalidCredentials
	}
	var encoded sql.NullString
	if err := s.DB.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id=? AND status='active'`, state.User.ID).Scan(&encoded); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return time.Time{}, ErrUnauthenticated
		}
		return time.Time{}, err
	}
	if !encoded.Valid || !VerifyPassword(encoded.String, password) {
		return time.Time{}, ErrInvalidCredentials
	}
	hash, _ := tokenHash(token)
	now := time.Now().UTC()
	result, err := s.DB.ExecContext(ctx, `UPDATE sessions SET reauthenticated_at=UTC_TIMESTAMP(6) WHERE token_hash=? AND user_id=? AND revoked_at IS NULL`, hash[:], state.User.ID)
	if err != nil {
		return time.Time{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return time.Time{}, ErrUnauthenticated
	}
	return now.Add(ReauthDuration), nil
}

func (s Service) RecentlyReauthenticated(ctx context.Context, token string) (bool, error) {
	if _, err := s.Peek(ctx, token); err != nil {
		return false, err
	}
	hash, _ := tokenHash(token)
	var at sql.NullTime
	if err := s.DB.QueryRowContext(ctx, `SELECT reauthenticated_at FROM sessions WHERE token_hash=? AND revoked_at IS NULL`, hash[:]).Scan(&at); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, ErrUnauthenticated
		}
		return false, err
	}
	return at.Valid && !time.Now().UTC().After(at.Time.Add(ReauthDuration)), nil
}
