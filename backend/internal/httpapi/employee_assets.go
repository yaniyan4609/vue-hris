package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

type employeeAssetRecord struct {
	ID              string     `json:"id"`
	EmployeeID      string     `json:"employee_id"`
	AssetID         string     `json:"asset_id"`
	AssetCode       string     `json:"asset_code"`
	AssetName       string     `json:"asset_name"`
	Status          string     `json:"status"`
	IssuedAt        time.Time  `json:"issued_at"`
	ReturnedAt      *time.Time `json:"returned_at"`
	IssueNote       *string    `json:"issue_note"`
	ReturnNote      *string    `json:"return_note"`
	ReturnCondition *string    `json:"return_condition"`
	IssuedBy        *string    `json:"issued_by"`
	ReturnedBy      *string    `json:"returned_by"`
	Version         uint64     `json:"version"`
}

const employeeAssetColumns = `x.id,x.employee_id,x.asset_id,s.code,s.name,x.issued_at,x.returned_at,x.issue_note,x.return_note,x.return_condition,x.issued_by,x.returned_by,x.version`
const employeeAssetJoin = ` FROM employee_assets x JOIN assets s ON s.tenant_id=x.tenant_id AND s.id=x.asset_id JOIN employees e ON e.tenant_id=x.tenant_id AND e.id=x.employee_id` + employeeScopeJoin

func scanEmployeeAsset(row interface{ Scan(...any) error }) (employeeAssetRecord, error) {
	var item employeeAssetRecord
	var returned sql.NullTime
	var issueNote, returnNote, condition, issuer, returner sql.NullString
	err := row.Scan(&item.ID, &item.EmployeeID, &item.AssetID, &item.AssetCode, &item.AssetName, &item.IssuedAt, &returned, &issueNote, &returnNote, &condition, &issuer, &returner, &item.Version)
	item.Status = "active"
	if returned.Valid {
		item.Status = "returned"
		item.ReturnedAt = &returned.Time
	}
	if issueNote.Valid {
		item.IssueNote = &issueNote.String
	}
	if returnNote.Valid {
		item.ReturnNote = &returnNote.String
	}
	if condition.Valid {
		item.ReturnCondition = &condition.String
	}
	if issuer.Valid {
		item.IssuedBy = &issuer.String
	}
	if returner.Valid {
		item.ReturnedBy = &returner.String
	}
	return item, err
}

func (a adminAPI) listEmployeeAssets(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "aset.lihat")
	if !ok {
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	scope, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	where := `x.tenant_id=? AND ` + scope
	args := append([]any{c.tenant.ID}, scopeArgs...)
	for _, f := range []struct{ name, col string }{{"employee_id", "x.employee_id"}, {"asset_id", "x.asset_id"}} {
		if id := r.URL.Query().Get(f.name); id != "" {
			if !fileIDPattern.MatchString(id) {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "ID filter tidak valid.")
				return
			}
			where += ` AND ` + f.col + `=?`
			args = append(args, id)
		}
	}
	if status := r.URL.Query().Get("status"); status != "" {
		if status != "active" && status != "returned" {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Status tidak valid.")
			return
		}
		if status == "active" {
			where += ` AND x.returned_at IS NULL`
		} else {
			where += ` AND x.returned_at IS NOT NULL`
		}
	}
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		if len([]rune(q)) > 100 {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Pencarian terlalu panjang.")
			return
		}
		where += ` AND (s.code LIKE ? OR s.name LIKE ? OR e.display_name LIKE ?)`
		args = append(args, "%"+q+"%", "%"+q+"%", "%"+q+"%")
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*)`+employeeAssetJoin+` WHERE `+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghitung peminjaman.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT `+employeeAssetColumns+employeeAssetJoin+` WHERE `+where+` ORDER BY x.issued_at DESC,x.id DESC LIMIT ? OFFSET ?`, append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca peminjaman.")
		return
	}
	defer rows.Close()
	items := []employeeAssetRecord{}
	for rows.Next() {
		item, e := scanEmployeeAsset(rows)
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca peminjaman.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca peminjaman.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}

func (a adminAPI) issueEmployeeAsset(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "aset.buat")
	if !ok {
		return
	}
	var in struct {
		EmployeeID string `json:"employee_id"`
		AssetID    string `json:"asset_id"`
		Note       string `json:"note"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	in.Note = strings.TrimSpace(in.Note)
	if !fileIDPattern.MatchString(in.EmployeeID) || !fileIDPattern.MatchString(in.AssetID) || len([]rune(in.Note)) > 500 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Karyawan, aset, atau catatan tidak valid.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyerahkan aset.")
		return
	}
	defer tx.Rollback()
	scope, args := access.EmployeeWhere(c.subject, c.effective)
	var employeeStatus string
	var deleted sql.NullTime
	err = tx.QueryRowContext(r.Context(), `SELECT e.status,e.deleted_at FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=1 WHERE e.tenant_id=? AND e.id=? AND `+scope+` FOR UPDATE`, append([]any{c.tenant.ID, in.EmployeeID}, args...)...).Scan(&employeeStatus, &deleted)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca karyawan.")
		return
	}
	if deleted.Valid || employeeStatus == "resigned" || employeeStatus == "terminated" {
		writeAPIError(w, r, 409, "EMPLOYEE_INACTIVE", "Karyawan tidak aktif.")
		return
	}
	var exiting int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM terminations WHERE tenant_id=? AND employee_id=? AND status='approved'`, c.tenant.ID, in.EmployeeID).Scan(&exiting); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa exit.")
		return
	}
	if exiting > 0 {
		writeAPIError(w, r, 409, "EXIT_APPROVED", "Terminasi karyawan sudah disetujui.")
		return
	}
	var cleared int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM terminations t JOIN termination_checklist c ON c.tenant_id=t.tenant_id AND c.termination_id=t.id AND c.item_code='assets_returned' AND c.is_done=TRUE WHERE t.tenant_id=? AND t.employee_id=? AND t.status IN ('draft','in_review','returned','approved')`, c.tenant.ID, in.EmployeeID).Scan(&cleared); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa checklist exit.")
		return
	}
	if cleared > 0 {
		writeAPIError(w, r, 409, "EXIT_ASSETS_CLEARED", "Checklist exit sudah menandai semua aset kembali.")
		return
	}
	var status string
	var assetVersion uint64
	err = tx.QueryRowContext(r.Context(), `SELECT status,version FROM assets WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, in.AssetID).Scan(&status, &assetVersion)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Aset tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca aset.")
		return
	}
	if status != "available" {
		writeAPIError(w, r, 409, "ASSET_UNAVAILABLE", "Aset sedang dipinjam, dirawat, atau diarsipkan.")
		return
	}
	var active int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM employee_assets WHERE tenant_id=? AND asset_id=? AND active_flag=1`, c.tenant.ID, in.AssetID).Scan(&active); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa peminjaman aktif.")
		return
	}
	if active > 0 {
		writeAPIError(w, r, 409, "ASSET_ASSIGNED", "Aset masih dipinjam.")
		return
	}
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID peminjaman.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO employee_assets (tenant_id,id,employee_id,asset_id,issued_at,active_flag,issued_by,issue_note) VALUES (?,?,?,?,UTC_TIMESTAMP(6),1,?,?)`, c.tenant.ID, id, in.EmployeeID, in.AssetID, c.state.User.ID, nullableString(in.Note)); err != nil {
		writeAPIError(w, r, 409, "ASSET_ASSIGNED", "Aset masih dipinjam.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE assets SET status='assigned',version=version+1 WHERE tenant_id=? AND id=? AND status='available'`, c.tenant.ID, in.AssetID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memperbarui aset.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "issue", "employee_assets", id, nil, map[string]any{"employee_id": in.EmployeeID, "asset_id": in.AssetID, "status": "active", "version": 1}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyerahkan aset.")
		return
	}
	writeAPIData(w, 201, map[string]any{"id": id, "asset_id": in.AssetID, "employee_id": in.EmployeeID, "status": "active", "version": 1, "asset_version": assetVersion + 1}, nil)
}

func (a adminAPI) returnEmployeeAsset(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "aset.ubah")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Peminjaman tidak ditemukan.")
		return
	}
	var in struct {
		Condition string `json:"return_condition"`
		Note      string `json:"note"`
		Version   uint64 `json:"version"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	in.Note = strings.TrimSpace(in.Note)
	if (in.Condition != "good" && in.Condition != "damaged") || in.Version == 0 || len([]rune(in.Note)) > 500 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Kondisi, catatan, atau versi tidak valid.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengembalikan aset.")
		return
	}
	defer tx.Rollback()
	var assetID string
	err = tx.QueryRowContext(r.Context(), `SELECT asset_id FROM employee_assets WHERE tenant_id=? AND id=?`, c.tenant.ID, id).Scan(&assetID)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Peminjaman tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca peminjaman.")
		return
	}
	var assetStatus string
	if err = tx.QueryRowContext(r.Context(), `SELECT status FROM assets WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, assetID).Scan(&assetStatus); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengunci aset.")
		return
	}
	scope, args := access.EmployeeWhere(c.subject, c.effective)
	item, err := scanEmployeeAsset(tx.QueryRowContext(r.Context(), `SELECT `+employeeAssetColumns+employeeAssetJoin+` WHERE x.tenant_id=? AND x.id=? AND `+scope+` FOR UPDATE`, append([]any{c.tenant.ID, id}, args...)...))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Peminjaman tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca peminjaman.")
		return
	}
	if item.Status != "active" {
		writeAPIError(w, r, 409, "ALREADY_RETURNED", "Aset sudah dikembalikan.")
		return
	}
	if item.Version != in.Version {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Peminjaman sudah berubah.")
		return
	}
	if assetStatus != "assigned" {
		writeAPIError(w, r, 409, "ASSET_STATE_CONFLICT", "Status aset tidak sesuai peminjaman.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE employee_assets SET returned_at=UTC_TIMESTAMP(6),active_flag=NULL,returned_by=?,return_condition=?,return_note=?,version=version+1 WHERE tenant_id=? AND id=? AND active_flag=1 AND version=?`, c.state.User.ID, in.Condition, nullableString(in.Note), c.tenant.ID, id, in.Version); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat pengembalian.")
		return
	}
	newStatus := "available"
	if in.Condition == "damaged" {
		newStatus = "maintenance"
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE assets SET status=?,version=version+1 WHERE tenant_id=? AND id=?`, newStatus, c.tenant.ID, assetID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memperbarui aset.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "return", "employee_assets", id, map[string]any{"status": "active", "version": item.Version}, map[string]any{"status": "returned", "return_condition": in.Condition, "version": item.Version + 1}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengembalikan aset.")
		return
	}
	writeAPIData(w, 200, map[string]any{"id": id, "asset_id": assetID, "status": "returned", "return_condition": in.Condition, "asset_status": newStatus, "version": item.Version + 1}, nil)
}
