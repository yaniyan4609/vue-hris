package auth

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"net"
	"strings"
	"time"
)

var ErrPasswordPolicy = errors.New("kata sandi baru minimal 8 karakter")
var ErrSessionNotFound = errors.New("sesi perangkat tidak ditemukan")

type SessionInfo struct {
	ID          string    `json:"id"`
	DeviceLabel string    `json:"deviceLabel"`
	IP          string    `json:"ip,omitempty"`
	Remember    bool      `json:"remember"`
	Current     bool      `json:"current"`
	CreatedAt   time.Time `json:"createdAt"`
	LastSeenAt  time.Time `json:"lastSeenAt"`
}

func deviceLabel(userAgent string) string {
	ua := strings.ToLower(userAgent)
	device := "Perangkat"
	switch {
	case strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad"):
		device = "iPhone/iPad"
	case strings.Contains(ua, "android"):
		device = "Android"
	case strings.Contains(ua, "windows"):
		device = "Windows"
	case strings.Contains(ua, "macintosh"):
		device = "Mac"
	case strings.Contains(ua, "linux"):
		device = "Linux"
	}
	browser := ""
	switch {
	case strings.Contains(ua, "edg/"):
		browser = "Edge"
	case strings.Contains(ua, "chrome/"):
		browser = "Chrome"
	case strings.Contains(ua, "firefox/"):
		browser = "Firefox"
	case strings.Contains(ua, "safari/"):
		browser = "Safari"
	}
	if browser != "" {
		return browser + " · " + device
	}
	return device
}

func (s Service) ActiveSessions(ctx context.Context, token string) ([]SessionInfo, error) {
	state, err := s.Current(ctx, token)
	if err != nil {
		return nil, err
	}
	hash, _ := tokenHash(token)
	rows, err := s.DB.QueryContext(ctx, `SELECT id, COALESCE(device_label,'Perangkat'), ip, remember, created_at, last_seen_at, token_hash
		FROM sessions WHERE user_id=? AND revoked_at IS NULL
		AND (expires_at IS NULL OR expires_at>UTC_TIMESTAMP(6))
		AND (remember=1 OR TIMESTAMPDIFF(SECOND,last_seen_at,UTC_TIMESTAMP(6))<=idle_timeout_minutes*60)
		ORDER BY last_seen_at DESC LIMIT 100`, state.User.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []SessionInfo{}
	for rows.Next() {
		var info SessionInfo
		var ip, tokenHashBytes []byte
		if err := rows.Scan(&info.ID, &info.DeviceLabel, &ip, &info.Remember, &info.CreatedAt, &info.LastSeenAt, &tokenHashBytes); err != nil {
			return nil, err
		}
		if len(ip) == net.IPv6len {
			info.IP = net.IP(ip).String()
		}
		info.Current = subtle.ConstantTimeCompare(tokenHashBytes, hash[:]) == 1
		result = append(result, info)
	}
	return result, rows.Err()
}

func (s Service) LogoutAll(ctx context.Context, token string) error {
	return s.LogoutAllExceptCurrent(ctx, token, false)
}

func (s Service) LogoutAllExceptCurrent(ctx context.Context, token string, exceptCurrent bool) error {
	state, err := s.Current(ctx, token)
	if err != nil {
		return err
	}
	query := `UPDATE sessions SET revoked_at=UTC_TIMESTAMP(6), revoked_reason='logout_all' WHERE user_id=? AND revoked_at IS NULL`
	args := []any{state.User.ID}
	if exceptCurrent {
		hash, _ := tokenHash(token)
		query += ` AND token_hash<>?`
		args = append(args, hash[:])
	}
	_, err = s.DB.ExecContext(ctx, query, args...)
	return err
}

func (s Service) RevokeSession(ctx context.Context, token, sessionID string) (bool, error) {
	state, err := s.Current(ctx, token)
	if err != nil {
		return false, err
	}
	var targetHash []byte
	err = s.DB.QueryRowContext(ctx, `SELECT token_hash FROM sessions WHERE id=? AND user_id=? AND revoked_at IS NULL`, sessionID, state.User.ID).Scan(&targetHash)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrSessionNotFound
	}
	if err != nil {
		return false, err
	}
	currentHash, _ := tokenHash(token)
	current := subtle.ConstantTimeCompare(targetHash, currentHash[:]) == 1
	result, err := s.DB.ExecContext(ctx, `UPDATE sessions SET revoked_at=UTC_TIMESTAMP(6), revoked_reason='device_revoked' WHERE id=? AND user_id=? AND revoked_at IS NULL`, sessionID, state.User.ID)
	if err != nil {
		return false, err
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return false, ErrSessionNotFound
	}
	return current, nil
}

func (s Service) ChangePassword(ctx context.Context, token, currentPassword, newPassword string) error {
	if len([]rune(newPassword)) < 8 || len(newPassword) > 1024 {
		return ErrPasswordPolicy
	}
	state, err := s.Current(ctx, token)
	if err != nil {
		return err
	}
	newHash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var currentHash sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id=? AND status='active' FOR UPDATE`, state.User.ID).Scan(&currentHash)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrUnauthenticated
	}
	if err != nil {
		return err
	}
	if !currentHash.Valid || !VerifyPassword(currentHash.String, currentPassword) || currentPassword == newPassword {
		return ErrInvalidCredentials
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET password_hash=? WHERE id=?`, newHash, state.User.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE sessions SET reauthenticated_at=NULL WHERE user_id=? AND revoked_at IS NULL`, state.User.ID); err != nil {
		return err
	}
	hash, _ := tokenHash(token)
	if _, err := tx.ExecContext(ctx, `UPDATE sessions SET revoked_at=UTC_TIMESTAMP(6), revoked_reason='password_changed' WHERE user_id=? AND token_hash<>? AND revoked_at IS NULL`, state.User.ID, hash[:]); err != nil {
		return err
	}
	return tx.Commit()
}
