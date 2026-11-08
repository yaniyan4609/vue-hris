package httpapi

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"net/http"
	"os"
	"regexp"
	"strings"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/cryptodata"
	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

var sensitiveNumber = regexp.MustCompile(`^[0-9]{1,32}$`)

type taxBPJSInput struct {
	Version   string  `json:"version"`
	ValidFrom string  `json:"valid_from"`
	PTKP      string  `json:"ptkp_status"`
	NPWP      *string `json:"npwp"`
	BPJSKes   *string `json:"bpjs_kes_no"`
	BPJSTK    *string `json:"bpjs_tk_no"`
}

func tenantMasterKey() ([]byte, string, error) {
	id := os.Getenv("TENANT_MASTER_KEY_ID")
	if id == "" {
		id = "local-v1"
	}
	key, err := base64.StdEncoding.DecodeString(os.Getenv("TENANT_MASTER_KEY"))
	if err != nil || len(key) != cryptodata.KeySize {
		return nil, "", errors.New("tenant master key unavailable")
	}
	return key, id, nil
}

func loadTenantDataKey(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, tenantID string, version uint32) ([]byte, error) {
	master, masterID, err := tenantMasterKey()
	if err != nil {
		return nil, err
	}
	var storedID string
	var wrapped []byte
	if err = q.QueryRowContext(ctx, `SELECT master_key_id,wrapped_data_key FROM tenant_crypto_keys WHERE tenant_id=? AND key_version=?`, tenantID, version).Scan(&storedID, &wrapped); err != nil {
		return nil, err
	}
	if storedID != masterID {
		return nil, errors.New("tenant key uses a different master key version")
	}
	return cryptodata.UnwrapKey(master, wrapped, storedID, tenantID, version)
}

// The tenant row is locked by the caller before creating the first data key.
func activeTenantDataKey(ctx context.Context, tx *sql.Tx, tenantID string) ([]byte, uint32, error) {
	var version uint32
	err := tx.QueryRowContext(ctx, `SELECT key_version FROM tenant_crypto_keys WHERE tenant_id=? AND active_slot=1`, tenantID).Scan(&version)
	if err == nil {
		key, err := loadTenantDataKey(ctx, tx, tenantID, version)
		return key, version, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, 0, err
	}
	master, masterID, err := tenantMasterKey()
	if err != nil {
		return nil, 0, err
	}
	key, err := cryptodata.NewDataKey()
	if err != nil {
		return nil, 0, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(key_version),0)+1 FROM tenant_crypto_keys WHERE tenant_id=?`, tenantID).Scan(&version); err != nil || version == 0 {
		return nil, 0, errors.New("cannot allocate tenant key version")
	}
	wrapped, err := cryptodata.WrapKey(master, key, masterID, tenantID, version)
	if err != nil {
		return nil, 0, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO tenant_crypto_keys (tenant_id,key_version,master_key_id,wrapped_data_key,status,active_slot) VALUES (?,?,?,?,'active',1)`, tenantID, version, masterID, wrapped)
	return key, version, err
}

func (a adminAPI) taxBPJSEmployeeScope(w http.ResponseWriter, r *http.Request, c adminContext, employeeID string, write bool) (string, []any, bool) {
	if !fileIDPattern.MatchString(employeeID) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan.")
		return "", nil, false
	}
	scope, args := access.EmployeeWhere(c.subject, c.effective)
	if write {
		subject, decision, err := a.access.Decide(r.Context(), c.tenant.ID, c.state.User.ID, c.tenant.Timezone, "karyawan.lihat_sensitif")
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa akses sensitif.")
			return "", nil, false
		}
		if !decision.Allowed {
			writeAPIError(w, r, 403, "PERMISSION_DENIED", "Hak data sensitif karyawan diperlukan.")
			return "", nil, false
		}
		other, otherArgs := access.EmployeeWhere(subject, decision)
		scope = "(" + scope + ") AND (" + other + ")"
		args = append(args, otherArgs...)
	}
	return scope, args, true
}

func (a adminAPI) getEmployeeTaxBPJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	c, ok := a.authorize(w, r, "karyawan.lihat_sensitif")
	if !ok {
		return
	}
	employeeID := chi.URLParam(r, "id")
	scope, args, ok := a.taxBPJSEmployeeScope(w, r, c, employeeID, false)
	if !ok {
		return
	}
	var matched int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE WHERE e.tenant_id=? AND e.id=? AND `+scope, append([]any{c.tenant.ID, employeeID}, args...)...).Scan(&matched); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa karyawan.")
		return
	}
	if matched == 0 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT id,npwp_enc,ptkp_status,bpjs_kes_no_enc,bpjs_tk_no_enc,key_version,DATE_FORMAT(valid_from,'%Y-%m-%d'),IFNULL(DATE_FORMAT(valid_to,'%Y-%m-%d'),'') FROM employee_tax_bpjs WHERE tenant_id=? AND employee_id=? ORDER BY valid_from DESC,id DESC`, c.tenant.ID, employeeID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca data pajak karyawan.")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, from, to string
		var npwp, kes, tk []byte
		var ptkp sql.NullString
		var version uint32
		if err = rows.Scan(&id, &npwp, &ptkp, &kes, &tk, &version, &from, &to); err != nil {
			break
		}
		key, keyErr := loadTenantDataKey(r.Context(), a.db, c.tenant.ID, version)
		if keyErr != nil {
			err = keyErr
			break
		}
		decrypt := func(field string, ciphertext []byte) *string {
			if ciphertext == nil {
				return nil
			}
			plain, decryptErr := cryptodata.DecryptField(key, ciphertext, c.tenant.ID, id, field, version)
			if decryptErr != nil {
				err = decryptErr
				return nil
			}
			value := string(plain)
			return &value
		}
		item := map[string]any{"id": id, "valid_from": from, "valid_to": nil, "ptkp_status": ptkp.String, "npwp": decrypt("npwp", npwp), "bpjs_kes_no": decrypt("bpjs_kes_no", kes), "bpjs_tk_no": decrypt("bpjs_tk_no", tk)}
		if err != nil {
			break
		}
		if to != "" {
			item["valid_to"] = to
		}
		items = append(items, item)
	}
	if err != nil || rows.Err() != nil {
		writeAPIError(w, r, 503, "SENSITIVE_DATA_UNAVAILABLE", "Data sensitif tidak dapat dibuka.")
		return
	}
	version := ""
	if len(items) != 0 {
		version = items[0]["id"].(string)
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat akses data sensitif.")
		return
	}
	defer tx.Rollback()
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "read", "employee_tax_bpjs", employeeID, nil, map[string]any{"version_count": len(items)}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat akses data sensitif.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat akses data sensitif.")
		return
	}
	writeAPIData(w, 200, map[string]any{"employee_id": employeeID, "version": version, "profiles": items}, nil)
}

func (a adminAPI) putEmployeeTaxBPJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	c, ok := a.authorize(w, r, "karyawan.ubah")
	if !ok {
		return
	}
	employeeID := chi.URLParam(r, "id")
	scope, args, ok := a.taxBPJSEmployeeScope(w, r, c, employeeID, true)
	if !ok {
		return
	}
	verified, err := a.auth.RecentlyReauthenticated(r.Context(), cookieToken(r))
	if err != nil || !verified {
		writeAPIError(w, r, 403, "REAUTH_REQUIRED", "Verifikasi ulang sebelum mengubah data pajak karyawan.")
		return
	}
	var body taxBPJSInput
	if !readJSON(w, r, &body) {
		return
	}
	body.PTKP = strings.ToUpper(strings.TrimSpace(body.PTKP))
	if !validRuleDates(body.ValidFrom, nil) || !taxPTKPStatusPattern.MatchString(body.PTKP) || (body.Version != "" && !fileIDPattern.MatchString(body.Version)) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal, versi, atau status PTKP tidak valid.")
		return
	}
	for _, value := range []*string{body.NPWP, body.BPJSKes, body.BPJSTK} {
		if value != nil && !sensitiveNumber.MatchString(*value) {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Nomor sensitif harus berisi 1–32 digit.")
			return
		}
	}
	if _, _, err = tenantMasterKey(); err != nil {
		writeAPIError(w, r, 503, "SENSITIVE_DATA_UNAVAILABLE", "Kunci enkripsi tenant belum tersedia.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan data pajak karyawan.")
		return
	}
	defer tx.Rollback()
	var locked string
	if err = tx.QueryRowContext(r.Context(), `SELECT id FROM tenants WHERE id=? FOR UPDATE`, c.tenant.ID).Scan(&locked); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengunci tenant.")
		return
	}
	if err = tx.QueryRowContext(r.Context(), `SELECT e.id FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE WHERE e.tenant_id=? AND e.id=? AND `+scope+` FOR UPDATE`, append([]any{c.tenant.ID, employeeID}, args...)...).Scan(&locked); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope.")
		} else {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa karyawan.")
		}
		return
	}
	var priorID, priorFrom, priorTo string
	err = tx.QueryRowContext(r.Context(), `SELECT id,DATE_FORMAT(valid_from,'%Y-%m-%d'),IFNULL(DATE_FORMAT(valid_to,'%Y-%m-%d'),'') FROM employee_tax_bpjs WHERE tenant_id=? AND employee_id=? ORDER BY valid_from DESC,id DESC LIMIT 1 FOR UPDATE`, c.tenant.ID, employeeID).Scan(&priorID, &priorFrom, &priorTo)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca versi data pajak.")
		return
	}
	if body.Version != priorID {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Data pajak karyawan sudah berubah.")
		return
	}
	if priorID != "" && body.ValidFrom <= priorFrom {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Tanggal berlaku harus setelah versi terakhir.")
		return
	}
	var finalized int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM payroll_runs pr JOIN payroll_periods pp ON pp.tenant_id=pr.tenant_id AND pp.id=pr.period_id WHERE pr.tenant_id=? AND pr.status='finalized' AND pp.end_date>=?`, c.tenant.ID, body.ValidFrom).Scan(&finalized); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa payroll final.")
		return
	}
	if finalized != 0 {
		writeAPIError(w, r, 409, "PAYROLL_LOCKED", "Tanggal berlaku menyentuh payroll final.")
		return
	}
	key, keyVersion, err := activeTenantDataKey(r.Context(), tx, c.tenant.ID)
	if err != nil {
		writeAPIError(w, r, 503, "SENSITIVE_DATA_UNAVAILABLE", "Kunci enkripsi tenant tidak tersedia.")
		return
	}
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID data pajak.")
		return
	}
	encrypt := func(field string, value *string) ([]byte, error) {
		if value == nil {
			return nil, nil
		}
		return cryptodata.EncryptField(key, c.tenant.ID, id, field, keyVersion, []byte(*value))
	}
	npwp, err := encrypt("npwp", body.NPWP)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengenkripsi NPWP.")
		return
	}
	kes, err := encrypt("bpjs_kes_no", body.BPJSKes)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengenkripsi nomor BPJS.")
		return
	}
	tk, err := encrypt("bpjs_tk_no", body.BPJSTK)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengenkripsi nomor BPJS.")
		return
	}
	if priorID != "" && (priorTo == "" || priorTo >= body.ValidFrom) {
		if _, err = tx.ExecContext(r.Context(), `UPDATE employee_tax_bpjs SET valid_to=? WHERE tenant_id=? AND id=?`, dateBefore(body.ValidFrom), c.tenant.ID, priorID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menutup versi data pajak.")
			return
		}
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO employee_tax_bpjs (tenant_id,id,employee_id,npwp_enc,ptkp_status,bpjs_kes_no_enc,bpjs_tk_no_enc,key_version,valid_from) VALUES (?,?,?,?,?,?,?,?,?)`, c.tenant.ID, id, employeeID, npwp, body.PTKP, kes, tk, keyVersion, body.ValidFrom); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan data pajak.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "replace", "employee_tax_bpjs", id, map[string]any{"previous_id": priorID}, map[string]any{"employee_id": employeeID, "valid_from": body.ValidFrom, "ptkp_status": body.PTKP, "npwp_present": body.NPWP != nil, "bpjs_kes_present": body.BPJSKes != nil, "bpjs_tk_present": body.BPJSTK != nil}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit data pajak.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan data pajak.")
		return
	}
	writeAPIData(w, 200, map[string]any{"employee_id": employeeID, "version": id, "valid_from": body.ValidFrom}, nil)
}
