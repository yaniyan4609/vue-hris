package httpapi

import (
	"database/sql"
	"errors"
	"net/http"

	"digaji.id/hris/internal/ids"
)

func (a adminAPI) getEssResignation(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	item, err := scanTermination(a.db.QueryRowContext(r.Context(), `SELECT `+terminationColumns+` FROM terminations x WHERE x.tenant_id=? AND x.employee_id=? AND x.termination_type='resign' ORDER BY x.created_at DESC,x.id DESC LIMIT 1`, c.state.SelectedTenantID, c.employeeID))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIData(w, 200, nil, nil)
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca resign.")
		return
	}
	writeAPIData(w, 200, item, nil)
}

func (a adminAPI) submitEssResignation(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	var body struct {
		NoticeDate    *string `json:"notice_date"`
		EffectiveDate string  `json:"effective_date"`
		Reason        string  `json:"reason"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	zone := c.zone.String()
	in := terminationInput{EmployeeID: c.employeeID, Type: "resign", NoticeDate: body.NoticeDate, EffectiveDate: body.EffectiveDate, Reason: body.Reason}
	if !validTerminationInput(&in, zone) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal atau alasan tidak valid.")
		return
	}
	tenantID, actorID := c.state.SelectedTenantID, c.state.User.ID
	tx, err := a.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat resign.")
		return
	}
	defer tx.Rollback()
	var sourceID string
	if err = tx.QueryRowContext(r.Context(), `SELECT id FROM employee_assignments WHERE tenant_id=? AND employee_id=? AND current_flag=1 FOR UPDATE`, tenantID, c.employeeID).Scan(&sourceID); err != nil {
		writeAPIError(w, r, 409, "ASSIGNMENT_MISSING", "Penugasan aktif tidak tersedia.")
		return
	}
	var id, status, creator string
	err = tx.QueryRowContext(r.Context(), `SELECT id,status,created_by FROM terminations WHERE tenant_id=? AND employee_id=? AND status IN ('draft','in_review','returned','approved') FOR UPDATE`, tenantID, c.employeeID).Scan(&id, &status, &creator)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa resign.")
		return
	}
	if err == nil {
		if creator != actorID || status != "draft" && status != "returned" {
			writeAPIError(w, r, 409, "TERMINATION_ACTIVE", "Proses terminasi sudah aktif.")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `UPDATE terminations SET notice_date=?,effective_date=?,reason=?,version=version+1 WHERE tenant_id=? AND id=? AND source_assignment_id=?`, in.NoticeDate, in.EffectiveDate, in.Reason, tenantID, id, sourceID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah resign.")
			return
		}
	} else {
		id, err = ids.New()
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID.")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO terminations (tenant_id,id,employee_id,termination_type,notice_date,effective_date,reason,source_assignment_id,created_by) VALUES (?,?,?,'resign',?,?,?,?,?)`, tenantID, id, c.employeeID, in.NoticeDate, in.EffectiveDate, in.Reason, sourceID, actorID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan resign.")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO termination_checklist (tenant_id,termination_id,item_code) VALUES (?,?,'assets_returned'),(?,?,'handover_complete')`, tenantID, id, tenantID, id); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat checklist.")
			return
		}
	}
	if err = audit(r.Context(), tx, tenantID, actorID, "request_resign", "terminations", id, nil, map[string]any{"employee_id": c.employeeID, "effective_date": in.EffectiveDate, "status": status}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan resign.")
		return
	}
	a.submitTerminationRecord(w, r, tenantID, zone, actorID, c.employeeID, id)
}
