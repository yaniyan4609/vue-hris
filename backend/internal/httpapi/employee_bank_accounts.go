package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/cryptodata"
	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

var bankCodePattern = regexp.MustCompile(`^[A-Z0-9_]{2,32}$`)

type employeeBankAccount struct {
	ID            string  `json:"id"`
	BankCode      string  `json:"bank_code"`
	AccountNo     *string `json:"account_no"`
	AccountMasked string  `json:"account_masked"`
	HolderName    string  `json:"holder_name"`
	IsPrimary     bool    `json:"is_primary"`
	ValidFrom     string  `json:"valid_from"`
	ValidTo       *string `json:"valid_to"`
	RevokedAt     *string `json:"revoked_at"`
	Version       uint64  `json:"version"`
}

type bankAccountInput struct {
	BankCode   string  `json:"bank_code"`
	AccountNo  string  `json:"account_no"`
	HolderName string  `json:"holder_name"`
	IsPrimary  bool    `json:"is_primary"`
	ValidFrom  string  `json:"valid_from"`
	ValidTo    *string `json:"valid_to"`
}

type bankAccountPatch struct {
	Version    uint64          `json:"version"`
	BankCode   *string         `json:"bank_code"`
	AccountNo  *string         `json:"account_no"`
	HolderName *string         `json:"holder_name"`
	IsPrimary  *bool           `json:"is_primary"`
	ValidFrom  *string         `json:"valid_from"`
	ValidTo    json.RawMessage `json:"valid_to"`
}

func validBankAccount(input bankAccountInput, today string) bool {
	input.BankCode = strings.ToUpper(strings.TrimSpace(input.BankCode))
	input.HolderName = strings.TrimSpace(input.HolderName)
	if !bankCodePattern.MatchString(input.BankCode) || !sensitiveNumber.MatchString(input.AccountNo) || input.HolderName == "" || len([]rune(input.HolderName)) > 200 || !validRuleDates(input.ValidFrom, input.ValidTo) {
		return false
	}
	if input.IsPrimary && (input.ValidFrom > today || input.ValidTo != nil) {
		return false
	}
	return true
}

func maskedAccount(number string) string {
	if len(number) <= 4 {
		return strings.Repeat("•", len(number))
	}
	return strings.Repeat("•", len(number)-4) + number[len(number)-4:]
}

func tenantToday(zone string) string {
	location, err := time.LoadLocation(zone)
	if err != nil {
		location = time.UTC
	}
	return time.Now().In(location).Format("2006-01-02")
}

func (a adminAPI) bankAccountReader(w http.ResponseWriter, r *http.Request) (adminContext, string, bool, bool) {
	c, ok := a.authorize(w, r, "karyawan.lihat")
	if !ok {
		return c, "", false, false
	}
	employeeID, ok := a.scopedEmployee(w, r, c)
	if !ok {
		return c, "", false, false
	}
	subject, decision, err := a.access.Decide(r.Context(), c.tenant.ID, c.state.User.ID, c.tenant.Timezone, "karyawan.lihat_sensitif")
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa akses rekening sensitif.")
		return c, "", false, false
	}
	canSee := false
	if decision.Allowed {
		scope, args := access.EmployeeWhere(subject, decision)
		var count int
		err = a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM employees e`+employeeScopeJoin+` WHERE e.tenant_id=? AND e.id=? AND `+scope, append([]any{c.tenant.ID, employeeID}, args...)...).Scan(&count)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa scope rekening.")
			return c, "", false, false
		}
		canSee = count != 0
	}
	return c, employeeID, canSee, true
}

func (a adminAPI) bankAccountWriter(w http.ResponseWriter, r *http.Request) (adminContext, string, bool) {
	c, ok := a.authorize(w, r, "karyawan.ubah")
	if !ok {
		return c, "", false
	}
	employeeID := chi.URLParam(r, "id")
	scope, args, ok := a.taxBPJSEmployeeScope(w, r, c, employeeID, true)
	if !ok {
		return c, "", false
	}
	var count int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=1 WHERE e.tenant_id=? AND e.id=? AND e.deleted_at IS NULL AND `+scope, append([]any{c.tenant.ID, employeeID}, args...)...).Scan(&count); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa karyawan.")
		return c, "", false
	}
	if count == 0 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope.")
		return c, "", false
	}
	verified, err := a.auth.RecentlyReauthenticated(r.Context(), cookieToken(r))
	if err != nil || !verified {
		writeAPIError(w, r, 403, "REAUTH_REQUIRED", "Verifikasi ulang sebelum mengubah rekening karyawan.")
		return c, "", false
	}
	return c, employeeID, true
}

func (a adminAPI) listEmployeeBankAccounts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	c, employeeID, canSee, ok := a.bankAccountReader(w, r)
	if !ok {
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT id,bank_code,account_no_enc,holder_name_enc,key_version,is_primary,DATE_FORMAT(valid_from,'%Y-%m-%d'),IFNULL(DATE_FORMAT(valid_to,'%Y-%m-%d'),''),revoked_at,version FROM employee_bank_accounts WHERE tenant_id=? AND employee_id=? ORDER BY is_primary DESC,valid_from DESC,id DESC`, c.tenant.ID, employeeID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca rekening karyawan.")
		return
	}
	defer rows.Close()
	items := []employeeBankAccount{}
	for rows.Next() {
		var item employeeBankAccount
		var numberCipher, holderCipher []byte
		var keyVersion uint32
		var until string
		var revoked sql.NullTime
		if err = rows.Scan(&item.ID, &item.BankCode, &numberCipher, &holderCipher, &keyVersion, &item.IsPrimary, &item.ValidFrom, &until, &revoked, &item.Version); err != nil {
			break
		}
		key, e := loadTenantDataKey(r.Context(), a.db, c.tenant.ID, keyVersion)
		if e != nil {
			err = e
			break
		}
		number, e := cryptodata.DecryptField(key, numberCipher, c.tenant.ID, item.ID, "account_no", keyVersion)
		if e != nil {
			err = e
			break
		}
		holder, e := cryptodata.DecryptField(key, holderCipher, c.tenant.ID, item.ID, "holder_name", keyVersion)
		if e != nil {
			err = e
			break
		}
		item.AccountMasked = maskedAccount(string(number))
		item.HolderName = string(holder)
		if canSee {
			value := string(number)
			item.AccountNo = &value
		}
		if until != "" {
			item.ValidTo = &until
		}
		if revoked.Valid {
			value := revoked.Time.UTC().Format(time.RFC3339Nano)
			item.RevokedAt = &value
		}
		items = append(items, item)
	}
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		writeAPIError(w, r, 503, "SENSITIVE_DATA_UNAVAILABLE", "Rekening tidak dapat dibuka.")
		return
	}
	if canSee {
		tx, e := a.db.BeginTx(r.Context(), nil)
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat akses rekening.")
			return
		}
		defer tx.Rollback()
		if e = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "read", "employee_bank_accounts", employeeID, nil, map[string]any{"count": len(items)}); e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat akses rekening.")
			return
		}
		if e = tx.Commit(); e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat akses rekening.")
			return
		}
	}
	writeAPIData(w, 200, map[string]any{"employee_id": employeeID, "numbers_masked": !canSee, "accounts": items}, nil)
}

func lockBankEmployee(w http.ResponseWriter, r *http.Request, tx *sql.Tx, tenantID, employeeID string) bool {
	var id string
	err := tx.QueryRowContext(r.Context(), `SELECT id FROM tenants WHERE id=? FOR UPDATE`, tenantID).Scan(&id)
	if err == nil {
		err = tx.QueryRowContext(r.Context(), `SELECT id FROM employees WHERE tenant_id=? AND id=? AND deleted_at IS NULL FOR UPDATE`, tenantID, employeeID).Scan(&id)
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengunci rekening karyawan.")
		return false
	}
	return true
}

func encryptBankAccount(key []byte, keyVersion uint32, tenantID, recordID, number, holder string) ([]byte, []byte, error) {
	n, e := cryptodata.EncryptField(key, tenantID, recordID, "account_no", keyVersion, []byte(number))
	if e != nil {
		return nil, nil, e
	}
	h, e := cryptodata.EncryptField(key, tenantID, recordID, "holder_name", keyVersion, []byte(holder))
	return n, h, e
}

func (a adminAPI) createEmployeeBankAccount(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	c, employeeID, ok := a.bankAccountWriter(w, r)
	if !ok {
		return
	}
	var body bankAccountInput
	if !readJSON(w, r, &body) {
		return
	}
	body.BankCode = strings.ToUpper(strings.TrimSpace(body.BankCode))
	body.HolderName = strings.TrimSpace(body.HolderName)
	if !validBankAccount(body, tenantToday(c.tenant.Timezone)) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Data rekening atau tanggal berlaku tidak valid.")
		return
	}
	if _, _, err := tenantMasterKey(); err != nil {
		writeAPIError(w, r, 503, "SENSITIVE_DATA_UNAVAILABLE", "Kunci enkripsi tenant belum tersedia.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan rekening.")
		return
	}
	defer tx.Rollback()
	if !lockBankEmployee(w, r, tx, c.tenant.ID, employeeID) {
		return
	}
	key, keyVersion, err := activeTenantDataKey(r.Context(), tx, c.tenant.ID)
	if err != nil {
		writeAPIError(w, r, 503, "SENSITIVE_DATA_UNAVAILABLE", "Kunci enkripsi tenant tidak tersedia.")
		return
	}
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID rekening.")
		return
	}
	number, holder, err := encryptBankAccount(key, keyVersion, c.tenant.ID, id, body.AccountNo, body.HolderName)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengenkripsi rekening.")
		return
	}
	if body.IsPrimary {
		if _, err = tx.ExecContext(r.Context(), `UPDATE employee_bank_accounts SET is_primary=FALSE,primary_slot=NULL,version=version+1 WHERE tenant_id=? AND employee_id=? AND primary_slot=1`, c.tenant.ID, employeeID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengganti rekening utama.")
			return
		}
	}
	var slot any
	if body.IsPrimary {
		slot = 1
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO employee_bank_accounts (tenant_id,id,employee_id,bank_code,account_no_enc,holder_name_enc,key_version,is_primary,primary_slot,valid_from,valid_to) VALUES (?,?,?,?,?,?,?,?,?,?,?)`, c.tenant.ID, id, employeeID, body.BankCode, number, holder, keyVersion, body.IsPrimary, slot, body.ValidFrom, body.ValidTo); err != nil {
		writeAPIError(w, r, 409, "BANK_CONFLICT", "Rekening utama atau data rekening berubah.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "create", "employee_bank_accounts", id, nil, map[string]any{"employee_id": employeeID, "bank_code": body.BankCode, "is_primary": body.IsPrimary, "valid_from": body.ValidFrom}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit rekening.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan rekening.")
		return
	}
	writeAPIData(w, 201, map[string]any{"id": id, "employee_id": employeeID, "version": 1}, nil)
}

func (a adminAPI) patchEmployeeBankAccount(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	c, employeeID, ok := a.bankAccountWriter(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "accountId")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Rekening tidak ditemukan.")
		return
	}
	var body bankAccountPatch
	if !readJSON(w, r, &body) {
		return
	}
	if body.Version == 0 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Versi rekening wajib diisi.")
		return
	}
	if _, _, err := tenantMasterKey(); err != nil {
		writeAPIError(w, r, 503, "SENSITIVE_DATA_UNAVAILABLE", "Kunci enkripsi tenant belum tersedia.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah rekening.")
		return
	}
	defer tx.Rollback()
	if !lockBankEmployee(w, r, tx, c.tenant.ID, employeeID) {
		return
	}
	var old bankAccountInput
	var numberCipher, holderCipher []byte
	var oldKeyVersion uint32
	var oldVersion uint64
	var until string
	var revoked sql.NullTime
	err = tx.QueryRowContext(r.Context(), `SELECT bank_code,account_no_enc,holder_name_enc,key_version,is_primary,DATE_FORMAT(valid_from,'%Y-%m-%d'),IFNULL(DATE_FORMAT(valid_to,'%Y-%m-%d'),''),revoked_at,version FROM employee_bank_accounts WHERE tenant_id=? AND employee_id=? AND id=? FOR UPDATE`, c.tenant.ID, employeeID, id).Scan(&old.BankCode, &numberCipher, &holderCipher, &oldKeyVersion, &old.IsPrimary, &old.ValidFrom, &until, &revoked, &oldVersion)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Rekening tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca rekening.")
		return
	}
	if revoked.Valid {
		writeAPIError(w, r, 409, "BANK_REVOKED", "Rekening sudah dinonaktifkan.")
		return
	}
	if oldVersion != body.Version {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Rekening sudah berubah.")
		return
	}
	if until != "" {
		old.ValidTo = &until
	}
	oldKey, err := loadTenantDataKey(r.Context(), tx, c.tenant.ID, oldKeyVersion)
	if err != nil {
		writeAPIError(w, r, 503, "SENSITIVE_DATA_UNAVAILABLE", "Kunci rekening tidak tersedia.")
		return
	}
	number, err := cryptodata.DecryptField(oldKey, numberCipher, c.tenant.ID, id, "account_no", oldKeyVersion)
	if err != nil {
		writeAPIError(w, r, 503, "SENSITIVE_DATA_UNAVAILABLE", "Rekening tidak dapat dibuka.")
		return
	}
	holder, err := cryptodata.DecryptField(oldKey, holderCipher, c.tenant.ID, id, "holder_name", oldKeyVersion)
	if err != nil {
		writeAPIError(w, r, 503, "SENSITIVE_DATA_UNAVAILABLE", "Nama rekening tidak dapat dibuka.")
		return
	}
	old.AccountNo = string(number)
	old.HolderName = string(holder)
	next := old
	if body.BankCode != nil {
		next.BankCode = strings.ToUpper(strings.TrimSpace(*body.BankCode))
	}
	if body.AccountNo != nil {
		next.AccountNo = *body.AccountNo
	}
	if body.HolderName != nil {
		next.HolderName = strings.TrimSpace(*body.HolderName)
	}
	if body.IsPrimary != nil {
		next.IsPrimary = *body.IsPrimary
	}
	if body.ValidFrom != nil {
		next.ValidFrom = *body.ValidFrom
	}
	if body.ValidTo != nil {
		if string(body.ValidTo) == "null" {
			next.ValidTo = nil
		} else {
			var value string
			if err = json.Unmarshal(body.ValidTo, &value); err != nil {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal akhir tidak valid.")
				return
			}
			next.ValidTo = &value
		}
	}
	if !validBankAccount(next, tenantToday(c.tenant.Timezone)) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Data rekening atau tanggal berlaku tidak valid.")
		return
	}
	key, keyVersion, err := activeTenantDataKey(r.Context(), tx, c.tenant.ID)
	if err != nil {
		writeAPIError(w, r, 503, "SENSITIVE_DATA_UNAVAILABLE", "Kunci enkripsi tenant tidak tersedia.")
		return
	}
	numberCipher, holderCipher, err = encryptBankAccount(key, keyVersion, c.tenant.ID, id, next.AccountNo, next.HolderName)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengenkripsi rekening.")
		return
	}
	if next.IsPrimary && !old.IsPrimary {
		if _, err = tx.ExecContext(r.Context(), `UPDATE employee_bank_accounts SET is_primary=FALSE,primary_slot=NULL,version=version+1 WHERE tenant_id=? AND employee_id=? AND primary_slot=1`, c.tenant.ID, employeeID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengganti rekening utama.")
			return
		}
	}
	var slot any
	if next.IsPrimary {
		slot = 1
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE employee_bank_accounts SET bank_code=?,account_no_enc=?,holder_name_enc=?,key_version=?,is_primary=?,primary_slot=?,valid_from=?,valid_to=?,version=version+1 WHERE tenant_id=? AND employee_id=? AND id=?`, next.BankCode, numberCipher, holderCipher, keyVersion, next.IsPrimary, slot, next.ValidFrom, next.ValidTo, c.tenant.ID, employeeID, id); err != nil {
		writeAPIError(w, r, 409, "BANK_CONFLICT", "Rekening utama atau data rekening berubah.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "update", "employee_bank_accounts", id, map[string]any{"version": oldVersion}, map[string]any{"version": oldVersion + 1, "bank_code": next.BankCode, "is_primary": next.IsPrimary, "valid_from": next.ValidFrom}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit rekening.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah rekening.")
		return
	}
	writeAPIData(w, 200, map[string]any{"id": id, "employee_id": employeeID, "version": oldVersion + 1}, nil)
}

func (a adminAPI) revokeEmployeeBankAccount(w http.ResponseWriter, r *http.Request) {
	c, employeeID, ok := a.bankAccountWriter(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "accountId")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Rekening tidak ditemukan.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menonaktifkan rekening.")
		return
	}
	defer tx.Rollback()
	if !lockBankEmployee(w, r, tx, c.tenant.ID, employeeID) {
		return
	}
	var revoked sql.NullTime
	err = tx.QueryRowContext(r.Context(), `SELECT revoked_at FROM employee_bank_accounts WHERE tenant_id=? AND employee_id=? AND id=? FOR UPDATE`, c.tenant.ID, employeeID, id).Scan(&revoked)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Rekening tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca rekening.")
		return
	}
	if revoked.Valid {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE employee_bank_accounts SET revoked_at=UTC_TIMESTAMP(6),is_primary=FALSE,primary_slot=NULL,version=version+1 WHERE tenant_id=? AND employee_id=? AND id=?`, c.tenant.ID, employeeID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menonaktifkan rekening.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "revoke", "employee_bank_accounts", id, nil, map[string]any{"employee_id": employeeID}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit rekening.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menonaktifkan rekening.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
