package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"time"

	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

var leaveCodePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,47}$`)
var leaveDecimalPattern = regexp.MustCompile(`^-?[0-9]+(\.[0-9]{1,2})?$`)

type leaveTypeInput struct {
	Code               string `json:"code"`
	Name               string `json:"name"`
	IsPaid             bool   `json:"is_paid"`
	RequiresAttachment bool   `json:"requires_attachment"`
	MaxConsecutiveDays *int   `json:"max_consecutive_days"`
	IsActive           *bool  `json:"is_active"`
}

func validLeaveType(in *leaveTypeInput) bool {
	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	in.Name = strings.TrimSpace(in.Name)
	return leaveCodePattern.MatchString(in.Code) && in.Name != "" && len([]rune(in.Name)) <= 160 && (in.MaxConsecutiveDays == nil || (*in.MaxConsecutiveDays >= 1 && *in.MaxConsecutiveDays <= 366))
}

func (a adminAPI) listAdminLeaveTypes(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "cuti.kelola")
	if !ok {
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM leave_types WHERE tenant_id=?`, c.tenant.ID).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca jenis cuti.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT id,code,name,is_paid,requires_attachment,max_consecutive_days,is_active FROM leave_types WHERE tenant_id=? ORDER BY name,id LIMIT ? OFFSET ?`, c.tenant.ID, page.PageSize, page.Offset)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca jenis cuti.")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, code, name string
		var paid, attachment, active bool
		var maximum sql.NullInt64
		if err = rows.Scan(&id, &code, &name, &paid, &attachment, &maximum, &active); err != nil {
			break
		}
		var max any
		if maximum.Valid {
			max = maximum.Int64
		}
		items = append(items, map[string]any{"id": id, "code": code, "name": name, "is_paid": paid, "requires_attachment": attachment, "max_consecutive_days": max, "is_active": active})
	}
	if err != nil || rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca jenis cuti.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}

func (a adminAPI) createAdminLeaveType(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "cuti.kelola")
	if !ok {
		return
	}
	var in leaveTypeInput
	if !readJSON(w, r, &in) {
		return
	}
	if !validLeaveType(&in) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Kode, nama, atau batas hari cuti tidak valid.")
		return
	}
	id, err := ids.New()
	if err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	defer tx.Rollback()
	var duplicate int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM leave_types WHERE tenant_id=? AND code=?`, c.tenant.ID, in.Code).Scan(&duplicate); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if duplicate > 0 {
		writeAPIError(w, r, 409, "LEAVE_TYPE_EXISTS", "Kode jenis cuti sudah dipakai.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO leave_types (tenant_id,id,code,name,is_paid,requires_attachment,max_consecutive_days,is_active) VALUES (?,?,?,?,?,?,?,TRUE)`, c.tenant.ID, id, in.Code, in.Name, in.IsPaid, in.RequiresAttachment, in.MaxConsecutiveDays); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "create", "leave_types", id, nil, map[string]any{"code": in.Code, "name": in.Name}); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if err = tx.Commit(); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	writeAPIData(w, 201, map[string]any{"id": id, "code": in.Code, "name": in.Name, "is_paid": in.IsPaid, "requires_attachment": in.RequiresAttachment, "max_consecutive_days": in.MaxConsecutiveDays, "is_active": true}, nil)
}

func (a adminAPI) updateAdminLeaveType(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "cuti.kelola")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Jenis cuti tidak ditemukan.")
		return
	}
	var in leaveTypeInput
	if !readJSON(w, r, &in) {
		return
	}
	if !validLeaveType(&in) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Kirim representasi lengkap jenis cuti yang valid.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	defer tx.Rollback()
	var oldCode, oldName string
	var oldActive bool
	if err = tx.QueryRowContext(r.Context(), `SELECT code,name,is_active FROM leave_types WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, id).Scan(&oldCode, &oldName, &oldActive); errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Jenis cuti tidak ditemukan.")
		return
	} else if err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	active := oldActive
	if in.IsActive != nil {
		active = *in.IsActive
	}
	var duplicate int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM leave_types WHERE tenant_id=? AND code=? AND id<>?`, c.tenant.ID, in.Code, id).Scan(&duplicate); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if duplicate > 0 {
		writeAPIError(w, r, 409, "LEAVE_TYPE_EXISTS", "Kode jenis cuti sudah dipakai.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE leave_types SET code=?,name=?,is_paid=?,requires_attachment=?,max_consecutive_days=?,is_active=? WHERE tenant_id=? AND id=?`, in.Code, in.Name, in.IsPaid, in.RequiresAttachment, in.MaxConsecutiveDays, active, c.tenant.ID, id); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "update", "leave_types", id, map[string]any{"code": oldCode, "name": oldName, "is_active": oldActive}, map[string]any{"code": in.Code, "name": in.Name, "is_active": active}); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if err = tx.Commit(); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	writeAPIData(w, 200, map[string]any{"id": id, "code": in.Code, "name": in.Name, "is_paid": in.IsPaid, "requires_attachment": in.RequiresAttachment, "max_consecutive_days": in.MaxConsecutiveDays, "is_active": active}, nil)
}

func (a adminAPI) deactivateAdminLeaveType(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "cuti.kelola")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Jenis cuti tidak ditemukan.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	defer tx.Rollback()
	var active bool
	if err = tx.QueryRowContext(r.Context(), `SELECT is_active FROM leave_types WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, id).Scan(&active); errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Jenis cuti tidak ditemukan.")
		return
	} else if err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE leave_types SET is_active=FALSE WHERE tenant_id=? AND id=?`, c.tenant.ID, id); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "deactivate", "leave_types", id, map[string]any{"is_active": active}, map[string]any{"is_active": false}); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if err = tx.Commit(); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	w.WriteHeader(204)
}

type leavePolicyInput struct {
	LeaveTypeID   string          `json:"leave_type_id"`
	AccrualType   string          `json:"accrual_type"`
	QuotaDays     *json.Number    `json:"quota_days"`
	ValidFrom     string          `json:"valid_from"`
	ValidTo       *string         `json:"valid_to"`
	CarryOverDays *json.Number    `json:"carry_over_days"`
	ExpireMonths  *int            `json:"expire_months"`
	Eligibility   json.RawMessage `json:"eligibility_json"`
}

func validPolicy(in leavePolicyInput) bool {
	from, e := time.Parse("2006-01-02", in.ValidFrom)
	if len(in.LeaveTypeID) != 26 || e != nil || from.Format("2006-01-02") != in.ValidFrom || (in.AccrualType != "manual" && in.AccrualType != "annual") || in.CarryOverDays != nil || in.ExpireMonths != nil {
		return false
	}
	if in.AccrualType == "annual" && (from.Month() != time.January || from.Day() != 1 || in.QuotaDays == nil) {
		return false
	}
	if in.ValidTo != nil {
		to, e := time.Parse("2006-01-02", *in.ValidTo)
		if e != nil || to.Format("2006-01-02") != *in.ValidTo || to.Before(from) {
			return false
		}
		if in.AccrualType == "annual" && (to.Month() != time.December || to.Day() != 31) {
			return false
		}
	}
	if len(in.Eligibility) > 0 && string(in.Eligibility) != "null" && string(in.Eligibility) != "{}" {
		return false
	}
	if in.QuotaDays != nil {
		hundredths, ok := leaveHundredths(in.QuotaDays.String())
		if !ok || hundredths < 0 || hundredths > 9999999 || (in.AccrualType == "annual" && hundredths < 100) {
			return false
		}
	}
	return true
}

func (a adminAPI) listAdminLeavePolicies(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "cuti.kelola")
	if !ok {
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	typeID := r.URL.Query().Get("leave_type_id")
	if typeID != "" && len(typeID) != 26 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Jenis cuti tidak valid.")
		return
	}
	where := "tenant_id=?"
	args := []any{c.tenant.ID}
	if typeID != "" {
		where += " AND leave_type_id=?"
		args = append(args, typeID)
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM leave_policies WHERE `+where, args...).Scan(&total); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT id,leave_type_id,accrual_type,CAST(quota_days AS CHAR),valid_from,valid_to FROM leave_policies WHERE `+where+` ORDER BY valid_from DESC,id DESC LIMIT ? OFFSET ?`, append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, typeID, accrual string
		var quota sql.NullString
		var from time.Time
		var to sql.NullTime
		if err = rows.Scan(&id, &typeID, &accrual, &quota, &from, &to); err != nil {
			break
		}
		var quotaValue, toValue any
		if quota.Valid {
			quotaValue = quota.String
		}
		if to.Valid {
			toValue = to.Time.Format("2006-01-02")
		}
		items = append(items, map[string]any{"id": id, "leave_type_id": typeID, "accrual_type": accrual, "quota_days": quotaValue, "valid_from": from.Format("2006-01-02"), "valid_to": toValue})
	}
	if err != nil || rows.Err() != nil {
		writeLeaveIssue(w, r, fmt.Errorf("policy rows: %v %v", err, rows.Err()))
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}

func (a adminAPI) createAdminLeavePolicy(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "cuti.kelola")
	if !ok {
		return
	}
	var in leavePolicyInput
	if !readJSON(w, r, &in) {
		return
	}
	if !validPolicy(in) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Kebijakan manual atau annual harus valid; annual memerlukan kuota minimal 1 hari dan batas tahun kalender.")
		return
	}
	id, err := ids.New()
	if err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	defer tx.Rollback()
	var active bool
	if err = tx.QueryRowContext(r.Context(), `SELECT is_active FROM leave_types WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, in.LeaveTypeID).Scan(&active); errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "LEAVE_TYPE_NOT_FOUND", "Jenis cuti aktif tidak ditemukan.")
		return
	} else if err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if !active {
		writeAPIError(w, r, 404, "LEAVE_TYPE_NOT_FOUND", "Jenis cuti aktif tidak ditemukan.")
		return
	}
	var overlap int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM leave_policies WHERE tenant_id=? AND leave_type_id=? AND (valid_to IS NULL OR valid_to>=?) AND (? IS NULL OR valid_from<=?)`, c.tenant.ID, in.LeaveTypeID, in.ValidFrom, in.ValidTo, in.ValidTo).Scan(&overlap); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if overlap > 0 {
		writeAPIError(w, r, 409, "POLICY_OVERLAP", "Masa berlaku kebijakan cuti bertumpang tindih.")
		return
	}
	var quota any
	if in.QuotaDays != nil {
		quota = in.QuotaDays.String()
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO leave_policies (tenant_id,id,leave_type_id,accrual_type,quota_days,valid_from,valid_to) VALUES (?,?,?,?,?,?,?)`, c.tenant.ID, id, in.LeaveTypeID, in.AccrualType, quota, in.ValidFrom, in.ValidTo); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "create", "leave_policies", id, nil, map[string]any{"leave_type_id": in.LeaveTypeID, "valid_from": in.ValidFrom, "valid_to": in.ValidTo}); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if err = tx.Commit(); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	writeAPIData(w, 201, map[string]any{"id": id, "leave_type_id": in.LeaveTypeID, "accrual_type": in.AccrualType, "quota_days": quota, "valid_from": in.ValidFrom, "valid_to": in.ValidTo}, nil)
}

func (a adminAPI) endAdminLeavePolicy(w http.ResponseWriter, r *http.Request) {
	a.closeAdminLeavePolicy(w, r, false)
}
func (a adminAPI) deactivateAdminLeavePolicy(w http.ResponseWriter, r *http.Request) {
	a.closeAdminLeavePolicy(w, r, true)
}

func (a adminAPI) closeAdminLeavePolicy(w http.ResponseWriter, r *http.Request, deactivate bool) {
	c, ok := a.authorize(w, r, "cuti.kelola")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Kebijakan cuti tidak ditemukan.")
		return
	}
	zone, err := time.LoadLocation(c.tenant.Timezone)
	if err != nil {
		zone = time.UTC
	}
	endDate := time.Now().In(zone).AddDate(0, 0, -1).Format("2006-01-02")
	if !deactivate {
		var body struct {
			ValidTo string `json:"valid_to"`
		}
		if !readJSON(w, r, &body) {
			return
		}
		endDate = body.ValidTo
	}
	end, e := time.Parse("2006-01-02", endDate)
	if e != nil || end.Format("2006-01-02") != endDate {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal akhir kebijakan tidak valid.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	defer tx.Rollback()
	var typeID, accrualType string
	var from time.Time
	var oldTo sql.NullTime
	if err = tx.QueryRowContext(r.Context(), `SELECT leave_type_id,accrual_type,valid_from,valid_to FROM leave_policies WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, id).Scan(&typeID, &accrualType, &from, &oldTo); errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Kebijakan cuti tidak ditemukan.")
		return
	} else if err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if end.Before(from) || (oldTo.Valid && end.After(oldTo.Time)) {
		writeAPIError(w, r, 409, "POLICY_DATE_CONFLICT", "Tanggal akhir harus berada dalam masa berlaku kebijakan saat ini.")
		return
	}
	if accrualType == "annual" && (end.Month() != time.December || end.Day() != 31) {
		writeAPIError(w, r, 409, "POLICY_DATE_CONFLICT", "Kebijakan annual hanya dapat diakhiri pada 31 Desember.")
		return
	}
	var pending int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM leave_requests WHERE tenant_id=? AND leave_type_id=? AND status IN ('submitted','in_review') AND end_date>?`, c.tenant.ID, typeID, endDate).Scan(&pending); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if pending > 0 {
		writeAPIError(w, r, 409, "POLICY_IN_USE", "Masih ada pengajuan aktif setelah tanggal akhir.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE leave_policies SET valid_to=? WHERE tenant_id=? AND id=?`, endDate, c.tenant.ID, id); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	var before any
	if oldTo.Valid {
		before = oldTo.Time.Format("2006-01-02")
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "end", "leave_policies", id, map[string]any{"valid_to": before}, map[string]any{"valid_to": endDate}); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if err = tx.Commit(); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if deactivate {
		w.WriteHeader(204)
	} else {
		writeAPIData(w, 200, map[string]any{"id": id, "valid_to": endDate}, nil)
	}
}

func leaveHundredths(raw string) (int64, bool) {
	if !leaveDecimalPattern.MatchString(raw) {
		return 0, false
	}
	value, ok := new(big.Rat).SetString(raw)
	if !ok {
		return 0, false
	}
	value.Mul(value, big.NewRat(100, 1))
	if !value.IsInt() || !value.Num().IsInt64() {
		return 0, false
	}
	return value.Num().Int64(), true
}

func leaveDecimal(hundredths int64) string {
	sign := ""
	if hundredths < 0 {
		sign = "-"
		hundredths = -hundredths
	}
	return fmt.Sprintf("%s%d.%02d", sign, hundredths/100, hundredths%100)
}

func (a adminAPI) adjustAdminLeaveBalance(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "cuti.kelola")
	if !ok {
		return
	}
	valid, err := a.auth.RecentlyReauthenticated(r.Context(), cookieToken(r))
	if err != nil || !valid {
		writeAPIError(w, r, 403, "REAUTH_REQUIRED", "Verifikasi ulang kata sandi sebelum menyesuaikan saldo.")
		return
	}
	var in struct {
		EmployeeID    string      `json:"employee_id"`
		LeaveTypeID   string      `json:"leave_type_id"`
		Days          json.Number `json:"days"`
		EffectiveDate string      `json:"effective_date"`
		Reason        string      `json:"reason"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	in.Reason = strings.TrimSpace(in.Reason)
	date, e := time.Parse("2006-01-02", in.EffectiveDate)
	amount, validAmount := leaveHundredths(in.Days.String())
	if len(in.EmployeeID) != 26 || len(in.LeaveTypeID) != 26 || e != nil || date.Format("2006-01-02") != in.EffectiveDate || !validAmount || amount == 0 || amount < -9999999 || amount > 9999999 || in.Reason == "" || len([]rune(in.Reason)) > 500 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Karyawan, jenis, jumlah dua desimal, tanggal, dan alasan wajib valid.")
		return
	}
	employee, err := scanEmployee(a.db.QueryRowContext(r.Context(), employeeSelect+` WHERE e.tenant_id=? AND e.id=?`, c.tenant.ID, in.EmployeeID))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	target, err := a.targetFor(r.Context(), c.tenant.ID, employee)
	if err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if !c.effective.Includes(c.subject, target) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	defer tx.Rollback()
	var locked string
	if err = tx.QueryRowContext(r.Context(), `SELECT id FROM employees WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, in.EmployeeID).Scan(&locked); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	var typeExists int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM leave_types WHERE tenant_id=? AND id=?`, c.tenant.ID, in.LeaveTypeID).Scan(&typeExists); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if typeExists != 1 {
		writeAPIError(w, r, 404, "LEAVE_TYPE_NOT_FOUND", "Jenis cuti tidak ditemukan.")
		return
	}
	year := date.Year()
	zone, zoneErr := time.LoadLocation(c.tenant.Timezone)
	if zoneErr != nil {
		zone = time.UTC
	}
	if year < time.Now().In(zone).Year() {
		var annualGrant int
		if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM leave_accrual_grants WHERE tenant_id=? AND employee_id=? AND leave_type_id=? AND period_year=?`, c.tenant.ID, in.EmployeeID, in.LeaveTypeID, year).Scan(&annualGrant); err != nil {
			writeLeaveIssue(w, r, err)
			return
		}
		if annualGrant > 0 {
			writeAPIError(w, r, 409, "ANNUAL_LEAVE_EXPIRED", "Saldo cuti tahunan tahun sebelumnya sudah ditutup.")
			return
		}
	}
	from, to := fmt.Sprintf("%04d-01-01", year), fmt.Sprintf("%04d-01-01", year+1)
	var balance, reserved int64
	if err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(CAST(ROUND(SUM(days)*100) AS SIGNED),0) FROM leave_balance_ledger WHERE tenant_id=? AND employee_id=? AND leave_type_id=? AND effective_date>=? AND effective_date<?`, c.tenant.ID, in.EmployeeID, in.LeaveTypeID, from, to).Scan(&balance); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(CAST(ROUND(SUM(a.days)*100) AS SIGNED),0) FROM leave_request_allocations a JOIN leave_requests lr ON lr.tenant_id=a.tenant_id AND lr.id=a.leave_request_id WHERE a.tenant_id=? AND lr.employee_id=? AND lr.leave_type_id=? AND a.period_year=? AND lr.status IN ('submitted','in_review')`, c.tenant.ID, in.EmployeeID, in.LeaveTypeID, year).Scan(&reserved); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if balance+amount-reserved < 0 {
		writeAPIError(w, r, 409, "INSUFFICIENT_LEAVE_BALANCE", "Penyesuaian akan membuat saldo tersedia negatif.")
		return
	}
	id, err := ids.New()
	if err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO leave_balance_ledger (tenant_id,id,employee_id,leave_type_id,entry_type,days,effective_date,ref_type,ref_id,note) VALUES (?,?,?,?,'adjustment',? ,?,'manual_adjustment',?,?)`, c.tenant.ID, id, in.EmployeeID, in.LeaveTypeID, leaveDecimal(amount), in.EffectiveDate, id, in.Reason); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO leave_balances (tenant_id,employee_id,leave_type_id,period_year,balance_days,computed_at) VALUES (?,?,?,?,?,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE balance_days=VALUES(balance_days),computed_at=VALUES(computed_at)`, c.tenant.ID, in.EmployeeID, in.LeaveTypeID, year, leaveDecimal(balance+amount)); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "adjust", "leave_balance_ledger", id, map[string]any{"balance_days": leaveDecimal(balance)}, map[string]any{"balance_days": leaveDecimal(balance + amount), "days": leaveDecimal(amount), "reason": in.Reason}); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if err = tx.Commit(); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	writeAPIData(w, 201, map[string]any{"id": id, "employee_id": in.EmployeeID, "leave_type_id": in.LeaveTypeID, "year": year, "days": leaveDecimal(amount), "balance_after": leaveDecimal(balance + amount)}, nil)
}
