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

type workSchedule struct {
	ID                   string     `json:"id"`
	EmployeeID           string     `json:"employee_id"`
	WorkDate             string     `json:"work_date"`
	DayType              string     `json:"day_type"`
	ShiftTemplateID      *string    `json:"shift_template_id"`
	OfficeID             *string    `json:"office_id"`
	SourcePatternID      *string    `json:"source_pattern_id"`
	SourcePatternVersion *int       `json:"source_pattern_version"`
	Status               string     `json:"status"`
	Version              uint64     `json:"version"`
	PublishedAt          *time.Time `json:"published_at"`
}

const rosterColumns = "w.id,w.employee_id,w.work_date,w.day_type,w.shift_template_id,w.office_id,w.source_pattern_id,w.source_pattern_version,w.status,w.version,w.published_at"
const rosterJoin = ` FROM work_schedules w JOIN employees e ON e.tenant_id=w.tenant_id AND e.id=w.employee_id JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE`

func (a adminAPI) mountWorkSchedules(r chi.Router, config Options) {
	r.Get("/work-schedules", a.listWorkSchedules)
	r.With(checkOrigin(config)).Post("/work-schedules", a.createWorkSchedule)
	r.With(checkOrigin(config)).Post("/work-schedules/bulk", a.bulkWorkSchedules)
	r.With(checkOrigin(config)).Post("/work-schedules/publish", a.publishWorkSchedules)
	r.With(checkOrigin(config)).Patch("/work-schedules/{id}", a.updateWorkSchedule)
	r.With(checkOrigin(config)).Delete("/work-schedules/{id}", a.cancelWorkSchedule)
	r.Get("/shift-instances", a.listShiftInstances)
	r.Get("/shift-instances/{id}", a.getShiftInstance)
	r.Get("/roster", a.rosterGrid)
	r.With(checkOrigin(config)).Post("/shift-instances/generate", a.generateShiftInstances)
	r.With(checkOrigin(config)).Post("/shift-instances/{id}/cancel", a.cancelShiftInstance)
	r.Get("/shift-swaps", a.listAdminShiftSwaps)
}
func scanWorkSchedule(row interface{ Scan(...any) error }) (workSchedule, error) {
	var item workSchedule
	var date time.Time
	var shift, office, pattern sql.NullString
	var patternVersion sql.NullInt64
	var published sql.NullTime
	err := row.Scan(&item.ID, &item.EmployeeID, &date, &item.DayType, &shift, &office, &pattern, &patternVersion, &item.Status, &item.Version, &published)
	if err != nil {
		return item, err
	}
	item.WorkDate = date.Format("2006-01-02")
	if shift.Valid {
		item.ShiftTemplateID = &shift.String
	}
	if office.Valid {
		item.OfficeID = &office.String
	}
	if pattern.Valid {
		item.SourcePatternID = &pattern.String
	}
	if patternVersion.Valid {
		value := int(patternVersion.Int64)
		item.SourcePatternVersion = &value
	}
	if published.Valid {
		item.PublishedAt = &published.Time
	}
	return item, nil
}
func (a adminAPI) readWorkSchedule(r *http.Request, c adminContext, id string) (workSchedule, error) {
	filter, args := access.EmployeeWhere(c.subject, c.effective)
	params := append([]any{c.tenant.ID, id}, args...)
	return scanWorkSchedule(a.db.QueryRowContext(r.Context(), "SELECT "+rosterColumns+rosterJoin+" WHERE w.tenant_id=? AND w.id=? AND "+filter, params...))
}
func validRosterDraft(item *workSchedule) bool {
	date, err := time.Parse("2006-01-02", item.WorkDate)
	if err != nil || date.Format("2006-01-02") != item.WorkDate || len(item.EmployeeID) != 26 {
		return false
	}
	if item.DayType == "off" {
		return item.ShiftTemplateID == nil && item.OfficeID == nil
	}
	return item.DayType == "shift" && item.ShiftTemplateID != nil && len(*item.ShiftTemplateID) == 26 && item.OfficeID != nil && len(*item.OfficeID) == 26
}
func (a adminAPI) rosterTargetAllowed(r *http.Request, c adminContext, item workSchedule) (bool, error) {
	e, err := scanEmployee(a.db.QueryRowContext(r.Context(), employeeSelect+` WHERE e.tenant_id=? AND e.id=?`, c.tenant.ID, item.EmployeeID))
	if err != nil {
		return false, err
	}
	target, err := a.targetFor(r.Context(), c.tenant.ID, e)
	if err != nil {
		return false, err
	}
	if !c.effective.Includes(c.subject, target) {
		return false, nil
	}
	if item.DayType == "shift" {
		var region string
		err = a.db.QueryRowContext(r.Context(), `SELECT region_id FROM offices WHERE tenant_id=? AND id=?`, c.tenant.ID, *item.OfficeID).Scan(&region)
		if err != nil {
			return false, err
		}
		target.BaseOfficeID = *item.OfficeID
		target.RegionID = region
		if !c.effective.Includes(c.subject, target) {
			return false, nil
		}
		var active bool
		err = a.db.QueryRowContext(r.Context(), `SELECT is_active FROM shift_templates WHERE tenant_id=? AND id=?`, c.tenant.ID, *item.ShiftTemplateID).Scan(&active)
		if err != nil {
			return false, err
		}
		if !active {
			return false, nil
		}
	}
	return true, nil
}
func rosterValidationError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan, kantor, atau shift tidak ditemukan dalam tenant.")
	} else if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memvalidasi roster.")
	} else {
		writeAPIError(w, r, 403, "SCOPE_DENIED", "Penugasan di luar scope atau shift tidak aktif.")
	}
}
func (a adminAPI) listWorkSchedules(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "jadwal.lihat")
	if !ok {
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	filter, filterArgs := access.EmployeeWhere(c.subject, c.effective)
	where := "w.tenant_id=? AND " + filter
	args := append([]any{c.tenant.ID}, filterArgs...)
	for _, entry := range []struct{ query, column string }{{"employee_id", "w.employee_id"}, {"office_id", "w.office_id"}} {
		if value := r.URL.Query().Get(entry.query); value != "" {
			if len(value) != 26 {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Filter roster tidak valid.")
				return
			}
			where += " AND " + entry.column + "=?"
			args = append(args, value)
		}
	}
	for _, entry := range []struct{ query, operator string }{{"from", ">="}, {"to", "<="}} {
		if value := r.URL.Query().Get(entry.query); value != "" {
			date, err := time.Parse("2006-01-02", value)
			if err != nil || date.Format("2006-01-02") != value {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal filter tidak valid.")
				return
			}
			where += " AND w.work_date" + entry.operator + "?"
			args = append(args, value)
		}
	}
	if status := r.URL.Query().Get("status"); status != "" {
		if status != "draft" && status != "published" && status != "cancelled" {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Status roster tidak valid.")
			return
		}
		where += " AND w.status=?"
		args = append(args, status)
	} else {
		where += " AND w.status<>'cancelled'"
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), "SELECT COUNT(*)"+rosterJoin+" WHERE "+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca roster.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), "SELECT "+rosterColumns+rosterJoin+" WHERE "+where+" ORDER BY w.work_date,w.employee_id LIMIT ? OFFSET ?", append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca roster.")
		return
	}
	defer rows.Close()
	items := []workSchedule{}
	for rows.Next() {
		item, err := scanWorkSchedule(rows)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca roster.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca roster.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}
func (a adminAPI) createWorkSchedule(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "jadwal.buat")
	if !ok {
		return
	}
	var body struct {
		EmployeeID      string  `json:"employee_id"`
		WorkDate        string  `json:"work_date"`
		DayType         string  `json:"day_type"`
		ShiftTemplateID *string `json:"shift_template_id"`
		OfficeID        *string `json:"office_id"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	item := workSchedule{EmployeeID: body.EmployeeID, WorkDate: body.WorkDate, DayType: body.DayType, ShiftTemplateID: body.ShiftTemplateID, OfficeID: body.OfficeID, Status: "draft", Version: 1}
	if !validRosterDraft(&item) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal, jenis hari, shift, atau kantor tidak valid.")
		return
	}
	allowed, err := a.rosterTargetAllowed(r, c, item)
	if err != nil || !allowed {
		rosterValidationError(w, r, err)
		return
	}
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat roster.")
		return
	}
	item.ID = id
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan roster.")
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO work_schedules (tenant_id,id,employee_id,work_date,shift_template_id,day_type,office_id,status,created_by,version) VALUES (?,?,?,?,?,? ,?,'draft',?,1)`, c.tenant.ID, id, item.EmployeeID, item.WorkDate, item.ShiftTemplateID, item.DayType, item.OfficeID, c.state.User.ID); err != nil {
		writeAPIError(w, r, 409, "ROSTER_CONFLICT", "Karyawan sudah memiliki roster pada tanggal itu.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "create", "work_schedules", id, nil, item); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan roster.")
		return
	}
	writeAPIData(w, 201, item, nil)
}
func (a adminAPI) updateWorkSchedule(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "jadwal.ubah")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	old, err := a.readWorkSchedule(r, c, id)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Roster tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca roster.")
		return
	}
	if old.Status != "draft" {
		writeAPIError(w, r, 409, "ROSTER_PUBLISHED", "Roster yang sudah diterbitkan tidak dapat diubah sebagai draft.")
		return
	}
	var body struct {
		Version         uint64  `json:"version"`
		WorkDate        *string `json:"work_date"`
		DayType         *string `json:"day_type"`
		ShiftTemplateID *string `json:"shift_template_id"`
		OfficeID        *string `json:"office_id"`
		Reason          string  `json:"reason"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if body.Version != old.Version {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Roster sudah berubah.")
		return
	}
	item := old
	if body.WorkDate != nil {
		item.WorkDate = *body.WorkDate
	}
	if body.DayType != nil {
		item.DayType = *body.DayType
	}
	if item.DayType == "off" {
		item.ShiftTemplateID = nil
		item.OfficeID = nil
	} else {
		if body.ShiftTemplateID != nil {
			item.ShiftTemplateID = body.ShiftTemplateID
		}
		if body.OfficeID != nil {
			item.OfficeID = body.OfficeID
		}
	}
	if !validRosterDraft(&item) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Data roster tidak valid.")
		return
	}
	allowed, err := a.rosterTargetAllowed(r, c, item)
	if err != nil || !allowed {
		rosterValidationError(w, r, err)
		return
	}
	reason := strings.TrimSpace(body.Reason)
	if reason == "" {
		reason = "Perubahan roster draft"
	}
	if len([]rune(reason)) > 500 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Alasan terlalu panjang.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan roster.")
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(r.Context(), `UPDATE work_schedules SET work_date=?,day_type=?,shift_template_id=?,office_id=?,version=version+1 WHERE tenant_id=? AND id=? AND version=? AND status='draft'`, item.WorkDate, item.DayType, item.ShiftTemplateID, item.OfficeID, c.tenant.ID, id, body.Version)
	if err != nil {
		writeAPIError(w, r, 409, "ROSTER_CONFLICT", "Karyawan sudah memiliki roster pada tanggal itu.")
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Roster sudah berubah.")
		return
	}
	item.Version++
	before, _ := json.Marshal(old)
	after, _ := json.Marshal(item)
	revisionID, _ := ids.New()
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO work_schedule_revisions (tenant_id,id,work_schedule_id,revision_no,before_json,after_json,actor_user_id,reason) VALUES (?,?,?,?,?,?,?,?)`, c.tenant.ID, revisionID, id, item.Version, string(before), string(after), c.state.User.ID, reason); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat revisi.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "update", "work_schedules", id, old, item); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan roster.")
		return
	}
	writeAPIData(w, 200, item, nil)
}
func (a adminAPI) cancelWorkSchedule(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "jadwal.hapus")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	old, err := a.readWorkSchedule(r, c, id)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Roster tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca roster.")
		return
	}
	if old.Status != "draft" {
		writeAPIError(w, r, 409, "ROSTER_PUBLISHED", "Hanya roster draft yang dapat dihapus.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membatalkan roster.")
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(r.Context(), `UPDATE work_schedules SET status='cancelled',version=version+1 WHERE tenant_id=? AND id=? AND version=? AND status='draft'`, c.tenant.ID, id, old.Version)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membatalkan roster.")
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Roster sudah berubah.")
		return
	}
	after := old
	after.Status = "cancelled"
	after.Version++
	beforeJSON, _ := json.Marshal(old)
	afterJSON, _ := json.Marshal(after)
	revisionID, _ := ids.New()
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO work_schedule_revisions (tenant_id,id,work_schedule_id,revision_no,before_json,after_json,actor_user_id,reason) VALUES (?,?,?,?,?,?,?,'Roster draft dibatalkan')`, c.tenant.ID, revisionID, id, after.Version, string(beforeJSON), string(afterJSON), c.state.User.ID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat revisi.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "delete", "work_schedules", id, old, after); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membatalkan roster.")
		return
	}
	w.WriteHeader(204)
}
