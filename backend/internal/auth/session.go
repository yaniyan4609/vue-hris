package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"net"
	"strings"
	"time"

	"digaji.id/hris/internal/ids"
)

var (
	ErrInvalidCredentials = errors.New("email atau kata sandi salah")
	ErrUnauthenticated    = errors.New("sesi tidak sah")
	ErrTenantUnavailable  = errors.New("perusahaan tidak tersedia untuk akun ini")
)

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type Tenant struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Admin    bool   `json:"admin"`
	ESS      bool   `json:"ess"`
	Timezone string `json:"-"`
}

type State struct {
	User             User     `json:"user"`
	Platform         bool     `json:"platform"`
	SelectedTenantID string   `json:"selectedTenantId,omitempty"`
	Tenants          []Tenant `json:"tenants"`
	Remember         bool     `json:"remember"`
}

type Service struct{ DB *sql.DB }

type SessionDevice struct {
	UserAgent string
	IP        net.IP
}

func (s Service) Login(ctx context.Context, email, password string, remember bool, device SessionDevice) (string, State, error) {
	var user User
	var hash sql.NullString
	err := s.DB.QueryRowContext(ctx, `SELECT id, email, display_name, password_hash FROM users WHERE email_normalized=? AND status='active'`, strings.ToLower(strings.TrimSpace(email))).Scan(&user.ID, &user.Email, &user.Name, &hash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", State{}, err
	}
	if err != nil || !hash.Valid || !VerifyPassword(hash.String, password) {
		if eventErr := logLoginEvent(ctx, s.DB, user.ID, "", "login_failure", device); eventErr != nil {
			return "", State{}, eventErr
		}
		return "", State{}, ErrInvalidCredentials
	}
	return s.createSession(ctx, user, remember, device, "login_success")
}

// DemoLogin bypasses the password only when the HTTP server enables staging
// demo mode. It never accepts an arbitrary email from the caller.
func (s Service) DemoLogin(ctx context.Context, account string, device SessionDevice) (string, State, error) {
	emails := map[string]string{
		"super": "super@digaji.id",
		"admin": "admin@digaji.id",
		"ess":   "ess@digaji.id",
	}
	email, ok := emails[account]
	if !ok {
		return "", State{}, ErrInvalidCredentials
	}
	var user User
	err := s.DB.QueryRowContext(ctx, `SELECT id, email, display_name FROM users WHERE email_normalized=? AND status='active'`, email).Scan(&user.ID, &user.Email, &user.Name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", State{}, ErrInvalidCredentials
		}
		return "", State{}, err
	}
	return s.createSession(ctx, user, false, device, "demo_login")
}

func (s Service) createSession(ctx context.Context, user User, remember bool, device SessionDevice, event string) (string, State, error) {
	token, hash, err := newToken()
	if err != nil {
		return "", State{}, err
	}
	id, err := ids.New()
	if err != nil {
		return "", State{}, err
	}
	state, err := s.stateForUser(ctx, user)
	if err != nil {
		return "", State{}, err
	}
	var selected any
	idleMinutes := 60
	if len(state.Tenants) == 1 {
		selected = state.Tenants[0].ID
		state.SelectedTenantID = state.Tenants[0].ID
		var configured int
		if err := s.DB.QueryRowContext(ctx, `SELECT idle_timeout_minutes FROM tenants WHERE id=?`, selected).Scan(&configured); err != nil {
			return "", State{}, err
		}
		idleMinutes = configured
	}
	userAgent := safeUserAgent(device.UserAgent)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", State{}, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO sessions (id, token_hash, user_id, selected_tenant_id, remember, idle_timeout_minutes, last_seen_at, user_agent, ip, device_label) VALUES (?, ?, ?, ?, ?, ?, UTC_TIMESTAMP(6), ?, ?, ?)`, id, hash[:], user.ID, selected, remember, idleMinutes, userAgent, deviceIP(device), deviceLabel(userAgent))
	if err != nil {
		return "", State{}, err
	}
	if err := logLoginEvent(ctx, tx, user.ID, state.SelectedTenantID, event, device); err != nil {
		return "", State{}, err
	}
	if err := tx.Commit(); err != nil {
		return "", State{}, err
	}
	state.Remember = remember
	return token, state, nil
}

func (s Service) Current(ctx context.Context, token string) (State, error) {
	hash, ok := tokenHash(token)
	if !ok {
		return State{}, ErrUnauthenticated
	}
	var user User
	var selected sql.NullString
	var remember bool
	var lastSeen time.Time
	var idleMinutes int
	var expires sql.NullTime
	err := s.DB.QueryRowContext(ctx, `SELECT u.id, u.email, u.display_name, s.selected_tenant_id, s.remember, s.last_seen_at, s.idle_timeout_minutes, s.expires_at FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=? AND s.revoked_at IS NULL AND u.status='active'`, hash[:]).Scan(&user.ID, &user.Email, &user.Name, &selected, &remember, &lastSeen, &idleMinutes, &expires)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return State{}, ErrUnauthenticated
		}
		return State{}, err
	}
	now := time.Now().UTC()
	if (!remember && now.Sub(lastSeen) > time.Duration(idleMinutes)*time.Minute) || (expires.Valid && now.After(expires.Time)) {
		_, _ = s.DB.ExecContext(ctx, `UPDATE sessions SET revoked_at=UTC_TIMESTAMP(6), revoked_reason='expired' WHERE token_hash=? AND revoked_at IS NULL`, hash[:])
		return State{}, ErrUnauthenticated
	}
	state, err := s.stateForUser(ctx, user)
	if err != nil {
		return State{}, err
	}
	state.Remember = remember
	if selected.Valid {
		for _, tenant := range state.Tenants {
			if tenant.ID == selected.String {
				state.SelectedTenantID = selected.String
				break
			}
		}
	}
	_, err = s.DB.ExecContext(ctx, `UPDATE sessions SET last_seen_at=UTC_TIMESTAMP(6) WHERE token_hash=? AND revoked_at IS NULL`, hash[:])
	return state, err
}

func (s Service) SelectTenant(ctx context.Context, token, tenantID string, device SessionDevice) (string, State, error) {
	state, err := s.Current(ctx, token)
	if err != nil {
		return "", State{}, err
	}
	var chosen *Tenant
	for i := range state.Tenants {
		if state.Tenants[i].ID == tenantID {
			chosen = &state.Tenants[i]
			break
		}
	}
	if chosen == nil {
		return "", State{}, ErrTenantUnavailable
	}
	newRaw, newHash, err := newToken()
	if err != nil {
		return "", State{}, err
	}
	oldHash, _ := tokenHash(token)
	var idle int
	if err := s.DB.QueryRowContext(ctx, `SELECT idle_timeout_minutes FROM tenants WHERE id=?`, tenantID).Scan(&idle); err != nil {
		return "", State{}, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", State{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE sessions SET selected_tenant_id=?, token_hash=?, idle_timeout_minutes=?, rotated_at=UTC_TIMESTAMP(6) WHERE token_hash=? AND revoked_at IS NULL`, tenantID, newHash[:], idle, oldHash[:])
	if err != nil {
		return "", State{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return "", State{}, ErrUnauthenticated
	}
	if err := logLoginEvent(ctx, tx, state.User.ID, tenantID, "tenant_selected", device); err != nil {
		return "", State{}, err
	}
	if err := tx.Commit(); err != nil {
		return "", State{}, err
	}
	state.SelectedTenantID = tenantID
	return newRaw, state, nil
}

func (s Service) Logout(ctx context.Context, token string) error {
	hash, ok := tokenHash(token)
	if !ok {
		return nil
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE sessions SET revoked_at=UTC_TIMESTAMP(6), revoked_reason='logout' WHERE token_hash=? AND revoked_at IS NULL`, hash[:])
	return err
}

func (s Service) stateForUser(ctx context.Context, user User) (State, error) {
	state := State{User: user, Tenants: []Tenant{}}
	platformDate := time.Now().In(mustLocation("Asia/Jakarta")).Format("2006-01-02")
	var platformCount int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM platform_user_roles pur JOIN platform_roles pr ON pr.id=pur.role_id WHERE pur.user_id=? AND pr.code='SUPERADMIN' AND pur.valid_from<=? AND (pur.valid_until IS NULL OR pur.valid_until>=?)`, user.ID, platformDate, platformDate).Scan(&platformCount)
	if err != nil {
		return State{}, err
	}
	state.Platform = platformCount > 0
	rows, err := s.DB.QueryContext(ctx, `SELECT t.id, t.name, t.timezone FROM tenant_memberships tm JOIN tenants t ON t.id=tm.tenant_id WHERE tm.user_id=? AND tm.status='active' AND t.status IN ('trial','active') ORDER BY t.name`, user.ID)
	if err != nil {
		return State{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var tenant Tenant
		if err := rows.Scan(&tenant.ID, &tenant.Name, &tenant.Timezone); err != nil {
			return State{}, err
		}
		state.Tenants = append(state.Tenants, tenant)
	}
	if err := rows.Err(); err != nil {
		return State{}, err
	}
	for i := range state.Tenants {
		tenant := &state.Tenants[i]
		date := time.Now().In(mustLocation(tenant.Timezone)).Format("2006-01-02")
		var override sql.NullString
		err := s.DB.QueryRowContext(ctx, `SELECT decision FROM user_permission_overrides WHERE tenant_id=? AND user_id=? AND permission_code='panel.admin'`, tenant.ID, user.ID).Scan(&override)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return State{}, err
		}
		if override.Valid && override.String == "allow" {
			tenant.Admin = true
		} else if !override.Valid || override.String != "deny" {
			var count int
			err = s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_roles ur JOIN role_permissions rp ON rp.tenant_id=ur.tenant_id AND rp.role_id=ur.role_id WHERE ur.tenant_id=? AND ur.user_id=? AND rp.permission_code='panel.admin' AND ur.valid_from<=? AND (ur.valid_until IS NULL OR ur.valid_until>=?)`, tenant.ID, user.ID, date, date).Scan(&count)
			if err != nil {
				return State{}, err
			}
			tenant.Admin = count > 0
		}
		var count int
		err = s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM employees WHERE tenant_id=? AND user_id=? AND status IN ('active','probation','leave')`, tenant.ID, user.ID).Scan(&count)
		if err != nil {
			return State{}, err
		}
		tenant.ESS = count > 0
	}
	return state, nil
}

func mustLocation(name string) *time.Location {
	location, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return location
}

func newToken() (string, [32]byte, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", [32]byte{}, err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), sha256.Sum256(raw[:]), nil
}

func tokenHash(encoded string) ([32]byte, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(raw) != 32 {
		return [32]byte{}, false
	}
	return sha256.Sum256(raw), true
}
