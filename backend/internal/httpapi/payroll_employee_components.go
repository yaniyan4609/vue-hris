package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

type employeePayComponentView struct {
	ID            string  `json:"id"`
	ComponentID   string  `json:"component_id"`
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	ComponentType string  `json:"component_type"`
	Amount        *string `json:"amount"`
	ValidFrom     string  `json:"valid_from"`
	ValidTo       *string `json:"valid_to"`
}

func (a adminAPI) payEmployeeScope(w http.ResponseWriter, r *http.Request, c adminContext, employeeID string, needSensitive bool) (string, []any, bool) {
	if !fileIDPattern.MatchString(employeeID) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope payroll.")
		return "", nil, false
	}
	scope, args := access.EmployeeWhere(c.subject, c.effective)
	if needSensitive {
		subject, decision, err := a.access.Decide(r.Context(), c.tenant.ID, c.state.User.ID, c.tenant.Timezone, "penggajian.lihat_sensitif")
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa hak payroll sensitif.")
			return "", nil, false
		}
		if !decision.Allowed {
			writeAPIError(w, r, 403, "PERMISSION_DENIED", "Hak melihat nominal gaji diperlukan.")
			return "", nil, false
		}
		additional, additionalArgs := access.EmployeeWhere(subject, decision)
		scope = "(" + scope + ") AND (" + additional + ")"
		args = append(args, additionalArgs...)
	}
	return scope, args, true
}

func (a adminAPI) getEmployeePayComponents(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "penggajian.lihat")
	if !ok {
		return
	}
	employeeID := chi.URLParam(r, "id")
	scope, args, ok := a.payEmployeeScope(w, r, c, employeeID, false)
	if !ok {
		return
	}
	var matched int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE WHERE e.tenant_id=? AND e.id=? AND `+scope, append([]any{c.tenant.ID, employeeID}, args...)...).Scan(&matched); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa karyawan.")
		return
	}
	if matched == 0 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope payroll.")
		return
	}
	subject, sensitive, err := a.access.Decide(r.Context(), c.tenant.ID, c.state.User.ID, c.tenant.Timezone, "penggajian.lihat_sensitif")
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa nominal gaji.")
		return
	}
	canSeeAmount := false
	if sensitive.Allowed {
		sensitiveScope, sensitiveArgs := access.EmployeeWhere(subject, sensitive)
		if err = a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE WHERE e.tenant_id=? AND e.id=? AND `+sensitiveScope, append([]any{c.tenant.ID, employeeID}, sensitiveArgs...)...).Scan(&matched); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa scope nominal gaji.")
			return
		}
		canSeeAmount = matched != 0
	}
	var version uint64
	err = a.db.QueryRowContext(r.Context(), `SELECT version FROM employee_pay_set_versions WHERE tenant_id=? AND employee_id=?`, c.tenant.ID, employeeID).Scan(&version)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca versi komponen gaji.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT ep.id,ep.component_id,pc.code,pc.name,pc.component_type,CAST(ep.amount AS CHAR),ep.valid_from,ep.valid_to FROM employee_pay_components ep JOIN pay_components pc ON pc.tenant_id=ep.tenant_id AND pc.id=ep.component_id WHERE ep.tenant_id=? AND ep.employee_id=? ORDER BY ep.valid_from DESC,pc.code,ep.id`, c.tenant.ID, employeeID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca komponen karyawan.")
		return
	}
	defer rows.Close()
	items := []employeePayComponentView{}
	for rows.Next() {
		var item employeePayComponentView
		var amount string
		var from time.Time
		var to sql.NullTime
		if err = rows.Scan(&item.ID, &item.ComponentID, &item.Code, &item.Name, &item.ComponentType, &amount, &from, &to); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca komponen karyawan.")
			return
		}
		item.ValidFrom = from.Format("2006-01-02")
		if to.Valid {
			formatted := to.Time.Format("2006-01-02")
			item.ValidTo = &formatted
		}
		if canSeeAmount {
			item.Amount = &amount
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca komponen karyawan.")
		return
	}
	writeAPIData(w, 200, map[string]any{"employee_id": employeeID, "version": version, "amounts_masked": !canSeeAmount, "components": items}, nil)
}

func (a adminAPI) putEmployeePayComponents(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "penggajian.kelola")
	if !ok {
		return
	}
	employeeID := chi.URLParam(r, "id")
	scope, scopeArgs, ok := a.payEmployeeScope(w, r, c, employeeID, true)
	if !ok {
		return
	}
	verified, err := a.auth.RecentlyReauthenticated(r.Context(), cookieToken(r))
	if err != nil || !verified {
		writeAPIError(w, r, 403, "REAUTH_REQUIRED", "Verifikasi ulang sebelum mengubah komponen gaji karyawan.")
		return
	}
	var body struct {
		ValidFrom  string `json:"valid_from"`
		Version    uint64 `json:"version"`
		Components []struct {
			ComponentID string      `json:"component_id"`
			Amount      json.Number `json:"amount"`
		} `json:"components"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	from, err := time.Parse("2006-01-02", body.ValidFrom)
	if err != nil || from.Year() < 2000 || from.Year() > 2100 || body.Components == nil || len(body.Components) > 100 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal atau daftar komponen tidak valid.")
		return
	}
	amounts := make(map[string]int64, len(body.Components))
	for _, component := range body.Components {
		amount, valid := leaveHundredths(component.Amount.String())
		if !fileIDPattern.MatchString(component.ComponentID) || !valid || amount <= 0 || len(strings.SplitN(component.Amount.String(), ".", 2)[0]) > 16 {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Nominal atau ID komponen tidak valid.")
			return
		}
		if _, exists := amounts[component.ComponentID]; exists {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Komponen gaji tidak boleh berulang.")
			return
		}
		amounts[component.ComponentID] = amount
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan komponen karyawan.")
		return
	}
	defer tx.Rollback()
	var locked string
	err = tx.QueryRowContext(r.Context(), `SELECT e.id FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE WHERE e.tenant_id=? AND e.id=? AND `+scope+` FOR UPDATE`, append([]any{c.tenant.ID, employeeID}, scopeArgs...)...).Scan(&locked)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope payroll.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa karyawan.")
		return
	}
	var currentVersion uint64
	err = tx.QueryRowContext(r.Context(), `SELECT version FROM employee_pay_set_versions WHERE tenant_id=? AND employee_id=? FOR UPDATE`, c.tenant.ID, employeeID).Scan(&currentVersion)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca versi komponen.")
		return
	}
	if body.Version != currentVersion {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Komponen karyawan sudah berubah.")
		return
	}
	var finalized, future int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM payroll_runs pr JOIN payroll_periods pp ON pp.tenant_id=pr.tenant_id AND pp.id=pr.period_id WHERE pr.tenant_id=? AND pr.status='finalized' AND pp.end_date>=?`, c.tenant.ID, body.ValidFrom).Scan(&finalized); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa periode final.")
		return
	}
	if finalized != 0 {
		writeAPIError(w, r, 409, "PAYROLL_LOCKED", "Tanggal berlaku menyentuh periode payroll final.")
		return
	}
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM employee_pay_components WHERE tenant_id=? AND employee_id=? AND valid_from>=?`, c.tenant.ID, employeeID, body.ValidFrom).Scan(&future); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa jadwal komponen.")
		return
	}
	if future != 0 {
		writeAPIError(w, r, 409, "FUTURE_ASSIGNMENT_EXISTS", "Sudah ada komponen mulai tanggal itu atau setelahnya; pilih tanggal baru.")
		return
	}
	for componentID := range amounts {
		var active bool
		var calcType string
		err = tx.QueryRowContext(r.Context(), `SELECT is_active,calc_type FROM pay_components WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, componentID).Scan(&active, &calcType)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && (!active || calcType != "fixed")) {
			writeAPIError(w, r, 409, "COMPONENT_UNAVAILABLE", "Komponen gaji tidak aktif atau belum mendukung nominal fixed.")
			return
		}
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca komponen gaji.")
			return
		}
	}
	previousDay := from.AddDate(0, 0, -1).Format("2006-01-02")
	if _, err = tx.ExecContext(r.Context(), `UPDATE employee_pay_components SET valid_to=? WHERE tenant_id=? AND employee_id=? AND valid_from<? AND (valid_to IS NULL OR valid_to>=?)`, previousDay, c.tenant.ID, employeeID, body.ValidFrom, body.ValidFrom); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menutup komponen lama.")
		return
	}
	for componentID, amount := range amounts {
		assignmentID, e := ids.New()
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID komponen karyawan.")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO employee_pay_components (tenant_id,id,employee_id,component_id,amount,valid_from) VALUES (?,?,?,?,?,?)`, c.tenant.ID, assignmentID, employeeID, componentID, leaveDecimal(amount), body.ValidFrom); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan komponen karyawan.")
			return
		}
	}
	if currentVersion == 0 {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO employee_pay_set_versions (tenant_id,employee_id,version,updated_by) VALUES (?,?,1,?)`, c.tenant.ID, employeeID, c.state.User.ID)
	} else {
		_, err = tx.ExecContext(r.Context(), `UPDATE employee_pay_set_versions SET version=version+1,updated_by=? WHERE tenant_id=? AND employee_id=?`, c.state.User.ID, c.tenant.ID, employeeID)
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memperbarui versi komponen.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "replace", "employee_pay_components", employeeID, map[string]any{"version": currentVersion}, map[string]any{"version": currentVersion + 1, "valid_from": body.ValidFrom, "component_count": len(amounts)}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat perubahan komponen karyawan.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan komponen karyawan.")
		return
	}
	writeAPIData(w, 200, map[string]any{"employee_id": employeeID, "version": currentVersion + 1, "valid_from": body.ValidFrom, "component_count": len(amounts)}, nil)
}
