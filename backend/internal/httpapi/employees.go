package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/auth"
	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

type employee struct {
	ID             string `json:"id"`
	EmployeeNumber string `json:"employeeNumber"`
	DisplayName    string `json:"displayName"`
	Status         string `json:"status"`
	BaseOfficeID   string `json:"baseOfficeId"`
	OfficeName     string `json:"officeName,omitempty"`
	PositionID     string `json:"positionId"`
	PositionName   string `json:"positionName,omitempty"`
	ReportsToID    string `json:"reportsToId,omitempty"`
	UserID         string `json:"userId,omitempty"`
}

// The latest assignment remains the scope snapshot after termination closes the current one.
const employeeScopeJoin = ` JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.id=(SELECT ax.id FROM employee_assignments ax WHERE ax.tenant_id=e.tenant_id AND ax.employee_id=e.id ORDER BY ax.current_flag DESC,ax.valid_from DESC,ax.id DESC LIMIT 1)`
const employeeFrom = ` FROM employees e` + employeeScopeJoin + ` JOIN offices o ON o.tenant_id=a.tenant_id AND o.id=a.office_id JOIN positions p ON p.tenant_id=a.tenant_id AND p.id=a.position_id`
const employeeSelect = `SELECT e.id,e.employee_number,e.display_name,e.status,a.office_id,o.name,a.position_id,p.name,COALESCE(a.reports_to_employee_id,''),COALESCE(e.user_id,'')` + employeeFrom

func scanEmployee(row interface{ Scan(...any) error }) (employee, error) {
	var e employee
	err := row.Scan(&e.ID, &e.EmployeeNumber, &e.DisplayName, &e.Status, &e.BaseOfficeID, &e.OfficeName, &e.PositionID, &e.PositionName, &e.ReportsToID, &e.UserID)
	return e, err
}

func (a adminAPI) listEmployees(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "karyawan.lihat")
	if !ok {
		return
	}
	page, ok := readPage(w, r)
	if !ok {
		return
	}
	q, ok := searchTerm(w, r)
	if !ok {
		return
	}
	sort := map[string]string{"name": "e.display_name", "number": "e.employee_number", "status": "e.status"}[r.URL.Query().Get("sort")]
	if sort == "" {
		if r.URL.Query().Get("sort") != "" {
			writeJSON(w, 400, map[string]string{"error": "Urutan tidak valid"})
			return
		}
		sort = "e.display_name"
	}
	direction := strings.ToLower(r.URL.Query().Get("order"))
	if direction == "" {
		direction = "asc"
	}
	if direction != "asc" && direction != "desc" {
		writeJSON(w, 400, map[string]string{"error": "Arah urutan tidak valid"})
		return
	}
	filter, args := access.EmployeeWhere(c.subject, c.effective)
	where := "e.tenant_id=? AND " + filter
	params := append([]any{c.tenant.ID}, args...)
	query := r.URL.Query()
	value := func(canonical, legacy string) string {
		if result := query.Get("filter[" + canonical + "]"); result != "" {
			return result
		}
		if result := query.Get(canonical); result != "" {
			return result
		}
		return query.Get(legacy)
	}
	switch archive := value("archive", ""); archive {
	case "", "active":
		where += " AND e.deleted_at IS NULL"
	case "archived":
		where += " AND e.deleted_at IS NOT NULL"
	case "all":
	default:
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Filter arsip tidak valid.")
		return
	}
	if q != "" {
		where += " AND (LOCATE(LOWER(?), LOWER(e.employee_number))>0 OR LOCATE(LOWER(?), LOWER(e.display_name))>0)"
		params = append(params, q, q)
	}
	if status := value("status", "status"); status != "" {
		if status != "active" && status != "probation" && status != "leave" && status != "resigned" && status != "terminated" {
			writeJSON(w, 400, map[string]string{"error": "Filter status tidak valid"})
			return
		}
		where += " AND e.status=?"
		params = append(params, status)
	}
	for _, dimension := range []struct{ key, legacy, column string }{
		{"office_id", "officeId", "a.office_id"}, {"region_id", "regionId", "a.region_id"},
		{"division_id", "divisionId", "a.division_id"}, {"department_id", "departmentId", "a.department_id"},
		{"position_id", "positionId", "a.position_id"}, {"reports_to_id", "reportsToId", "a.reports_to_employee_id"},
	} {
		if id := value(dimension.key, dimension.legacy); id != "" {
			if !fileIDPattern.MatchString(id) {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Filter organisasi tidak valid.")
				return
			}
			where += " AND " + dimension.column + "=?"
			params = append(params, id)
		}
	}
	joinedFrom, joinedTo := value("joined_from", "joinedFrom"), value("joined_to", "joinedTo")
	if joinedFrom != "" {
		if _, err := time.Parse("2006-01-02", joinedFrom); err != nil {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal masuk awal tidak valid.")
			return
		}
		where += " AND e.join_date>=?"
		params = append(params, joinedFrom)
	}
	if joinedTo != "" {
		if _, err := time.Parse("2006-01-02", joinedTo); err != nil || joinedFrom != "" && joinedTo < joinedFrom {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal masuk akhir tidak valid.")
			return
		}
		where += " AND e.join_date<=?"
		params = append(params, joinedTo)
	}
	if kind := value("contract_type", "contractType"); kind != "" {
		if kind != "PKWT" && kind != "PKWTT" && kind != "PROBASI" {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tipe kontrak tidak valid.")
			return
		}
		where += ` AND EXISTS (SELECT 1 FROM contracts ct WHERE ct.tenant_id=e.tenant_id AND ct.employee_id=e.id AND ct.contract_type=? AND ct.status IN ('active','draft'))`
		params = append(params, kind)
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), "SELECT COUNT(*)"+employeeFrom+" WHERE "+where, params...).Scan(&total); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca jumlah karyawan"})
		return
	}
	rows, err := a.db.QueryContext(r.Context(), employeeSelect+" WHERE "+where+" ORDER BY "+sort+" "+direction+", e.id ASC LIMIT ? OFFSET ?", append(params, page.PageSize, page.Offset)...)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca karyawan"})
		return
	}
	defer rows.Close()
	items := []employee{}
	for rows.Next() {
		e, err := scanEmployee(rows)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "Gagal membaca karyawan"})
			return
		}
		items = append(items, e)
	}
	if rows.Err() != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca karyawan"})
		return
	}
	writeJSON(w, 200, pageResult[employee]{Items: items, Total: total, Page: page.Page, PageSize: page.PageSize})
}

func (a adminAPI) getEmployee(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "karyawan.lihat")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		writeJSON(w, 404, map[string]string{"error": "Karyawan tidak ditemukan"})
		return
	}
	filter, args := access.EmployeeWhere(c.subject, c.effective)
	params := append([]any{c.tenant.ID, id}, args...)
	e, err := scanEmployee(a.db.QueryRowContext(r.Context(), employeeSelect+" WHERE e.tenant_id=? AND e.id=? AND "+filter, params...))
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, 404, map[string]string{"error": "Karyawan tidak ditemukan"})
		return
	}
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca karyawan"})
		return
	}
	a.employeePersonalDetail(w, r, c, id, e)
}

func validateEmployee(e employee) bool {
	if strings.TrimSpace(e.EmployeeNumber) == "" || len(e.EmployeeNumber) > 64 || strings.TrimSpace(e.DisplayName) == "" || len([]rune(e.DisplayName)) > 200 || len(e.BaseOfficeID) != 26 || len(e.PositionID) != 26 {
		return false
	}
	if e.ReportsToID != "" && len(e.ReportsToID) != 26 {
		return false
	}
	if e.UserID != "" && len(e.UserID) != 26 {
		return false
	}
	switch e.Status {
	// Resign and termination require their own dated workflow so roles and
	// sessions are revoked atomically on the effective date.
	case "active", "probation", "leave":
		return true
	}
	return false
}

func (a adminAPI) targetFor(ctx context.Context, tenantID string, e employee) (access.Target, error) {
	t := access.Target{TenantID: tenantID, EmployeeID: e.ID, UserID: e.UserID, ReportsToID: e.ReportsToID, BaseOfficeID: e.BaseOfficeID, PositionID: e.PositionID}
	err := a.db.QueryRowContext(ctx, `SELECT o.region_id,p.department_id,d.division_id FROM offices o JOIN regions r ON r.tenant_id=o.tenant_id AND r.id=o.region_id JOIN positions p ON p.tenant_id=o.tenant_id AND p.id=? JOIN departments d ON d.tenant_id=p.tenant_id AND d.id=p.department_id JOIN divisions v ON v.tenant_id=d.tenant_id AND v.id=d.division_id WHERE o.tenant_id=? AND o.id=? AND o.deleted_at IS NULL AND o.is_active=TRUE AND r.deleted_at IS NULL AND p.deleted_at IS NULL AND d.deleted_at IS NULL AND v.deleted_at IS NULL`, e.PositionID, tenantID, e.BaseOfficeID).Scan(&t.RegionID, &t.DepartmentID, &t.DivisionID)
	return t, err
}

func lockEmployeeOrganization(ctx context.Context, tx *sql.Tx, tenantID, officeID, positionID string) error {
	var found string
	return tx.QueryRowContext(ctx, `SELECT o.id FROM offices o JOIN regions r ON r.tenant_id=o.tenant_id AND r.id=o.region_id JOIN positions p ON p.tenant_id=o.tenant_id AND p.id=? JOIN departments d ON d.tenant_id=p.tenant_id AND d.id=p.department_id JOIN divisions v ON v.tenant_id=d.tenant_id AND v.id=d.division_id WHERE o.tenant_id=? AND o.id=? AND o.deleted_at IS NULL AND o.is_active=TRUE AND r.deleted_at IS NULL AND p.deleted_at IS NULL AND d.deleted_at IS NULL AND v.deleted_at IS NULL FOR UPDATE`, positionID, tenantID, officeID).Scan(&found)
}

func (a adminAPI) createEmployee(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "karyawan.buat")
	if !ok {
		return
	}
	var body employeeWrite
	if !readJSON(w, r, &body) {
		return
	}
	e := body.employee
	if e.UserID != "" {
		writeAPIError(w, r, 409, "ACCOUNT_WORKFLOW_REQUIRED", "Hubungkan akun melalui endpoint akun karyawan.")
		return
	}
	if err := validEmployeePersonal(body.employeePersonalWrite); err != nil {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", err.Error())
		return
	}
	e.EmployeeNumber = strings.TrimSpace(e.EmployeeNumber)
	e.DisplayName = strings.TrimSpace(e.DisplayName)
	if e.Status == "" {
		e.Status = "active"
	}
	if !validateEmployee(e) {
		writeJSON(w, 400, map[string]string{"error": "Data karyawan tidak valid"})
		return
	}
	target, err := a.targetFor(r.Context(), c.tenant.ID, e)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, 400, map[string]string{"error": "Kantor atau jabatan tidak tersedia"})
		return
	}
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal memvalidasi penempatan"})
		return
	}
	if !c.effective.Includes(c.subject, target) {
		writeJSON(w, 403, map[string]string{"error": "Penempatan di luar cakupan akses"})
		return
	}
	if !a.checkNIKWrite(w, r, c, "", &target, body.NIK) {
		return
	}
	id, err := ids.New()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membuat ID"})
		return
	}
	e.ID = id
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan karyawan"})
		return
	}
	defer tx.Rollback()
	var quota sql.NullInt64
	err = tx.QueryRowContext(r.Context(), `SELECT p.max_employees FROM tenants t JOIN tenant_subscriptions s ON s.tenant_id=t.id AND s.status='active' JOIN plans p ON p.id=s.plan_id WHERE t.id=? FOR UPDATE`, c.tenant.ID).Scan(&quota)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 409, "ENTITLEMENT_REQUIRED", "Paket aktif tidak tersedia.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa kuota karyawan.")
		return
	}
	if quota.Valid {
		var count int64
		if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM employees WHERE tenant_id=? AND deleted_at IS NULL AND status IN ('active','probation','leave')`, c.tenant.ID).Scan(&count); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghitung kuota karyawan.")
			return
		}
		if count >= quota.Int64 {
			writeAPIError(w, r, 422, "QUOTA_EXCEEDED", "Kuota karyawan paket telah tercapai.")
			return
		}
	}
	if err = lockEmployeeOrganization(r.Context(), tx, c.tenant.ID, e.BaseOfficeID, e.PositionID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, r, 409, "ORGANIZATION_UNAVAILABLE", "Kantor atau jabatan sudah diarsipkan.")
		} else {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa organisasi.")
		}
		return
	}
	_, err = tx.ExecContext(r.Context(), `INSERT INTO employees (tenant_id,id,user_id,employee_number,display_name,status,base_office_id,position_id,reports_to_id) VALUES (?,?,?,?,?,?,?,?,?)`, c.tenant.ID, id, nullID(e.UserID), e.EmployeeNumber, e.DisplayName, e.Status, e.BaseOfficeID, e.PositionID, nullID(e.ReportsToID))
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "Nomor karyawan duplikat atau referensi tidak tersedia"})
		return
	}
	if err = insertAssignment(r.Context(), tx, c.tenant.ID, e); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal mencatat penempatan"})
		return
	}
	if err = persistEmployeePersonal(r.Context(), tx, c.tenant.ID, id, body.employeePersonalWrite); err != nil {
		if err.Error() == "NIK_DUPLICATE" {
			writeAPIError(w, r, 409, "NIK_DUPLICATE", "NIK sudah digunakan.")
		} else {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Profil karyawan tidak dapat disimpan.")
		}
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "create", "employees", id, nil, e); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal mencatat audit"})
		return
	}
	if err = tx.Commit(); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan karyawan"})
		return
	}
	writeJSON(w, 201, e)
}

func (a adminAPI) updateEmployee(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "karyawan.ubah")
	if !ok {
		return
	}
	view, ok := a.authorize(w, r, "karyawan.lihat")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		writeJSON(w, 404, map[string]string{"error": "Karyawan tidak ditemukan"})
		return
	}
	filter, args := access.EmployeeWhere(view.subject, view.effective)
	params := append([]any{c.tenant.ID, id}, args...)
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan karyawan"})
		return
	}
	defer tx.Rollback()
	var tenantLock string
	if err = tx.QueryRowContext(r.Context(), `SELECT id FROM tenants WHERE id=? FOR UPDATE`, c.tenant.ID).Scan(&tenantLock); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengunci tenant.")
		return
	}
	old, err := scanEmployee(tx.QueryRowContext(r.Context(), employeeSelect+" WHERE e.tenant_id=? AND e.id=? AND e.deleted_at IS NULL AND "+filter+" FOR UPDATE", params...))
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, 404, map[string]string{"error": "Karyawan tidak ditemukan"})
		return
	}
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca karyawan"})
		return
	}
	var body employeeWrite
	if !readJSON(w, r, &body) {
		return
	}
	next := body.employee
	if err := validEmployeePersonal(body.employeePersonalWrite); err != nil {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", err.Error())
		return
	}
	if !a.checkNIKWrite(w, r, c, id, nil, body.NIK) {
		return
	}
	next.ID = id
	next.UserID = old.UserID
	if next.EmployeeNumber == "" {
		next.EmployeeNumber = old.EmployeeNumber
	}
	if next.DisplayName == "" {
		next.DisplayName = old.DisplayName
	}
	if next.Status == "" {
		next.Status = old.Status
	}
	if next.BaseOfficeID == "" {
		next.BaseOfficeID = old.BaseOfficeID
	}
	if next.PositionID == "" {
		next.PositionID = old.PositionID
	}
	if next.ReportsToID == "" {
		next.ReportsToID = old.ReportsToID
	}
	next.EmployeeNumber = strings.TrimSpace(next.EmployeeNumber)
	next.DisplayName = strings.TrimSpace(next.DisplayName)
	if !validateEmployee(next) {
		writeJSON(w, 400, map[string]string{"error": "Data karyawan tidak valid"})
		return
	}
	if next.BaseOfficeID != old.BaseOfficeID || next.PositionID != old.PositionID || next.ReportsToID != old.ReportsToID {
		writeAPIError(w, r, 409, "WORKFLOW_REQUIRED", "Perubahan penugasan memakai alur koreksi atau mutasi.")
		return
	}
	target, err := a.targetFor(r.Context(), c.tenant.ID, next)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, 400, map[string]string{"error": "Kantor atau jabatan tidak tersedia"})
		return
	}
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal memvalidasi penempatan"})
		return
	}
	if !c.effective.Includes(c.subject, target) || !view.effective.Includes(view.subject, target) {
		writeJSON(w, 403, map[string]string{"error": "Penempatan di luar cakupan akses"})
		return
	}
	if err = lockEmployeeOrganization(r.Context(), tx, c.tenant.ID, next.BaseOfficeID, next.PositionID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, r, 409, "ORGANIZATION_UNAVAILABLE", "Kantor atau jabatan sudah diarsipkan.")
		} else {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa organisasi.")
		}
		return
	}
	_, err = tx.ExecContext(r.Context(), `UPDATE employees SET employee_number=?,display_name=?,status=?,base_office_id=?,position_id=?,reports_to_id=?,version=version+1 WHERE tenant_id=? AND id=?`, next.EmployeeNumber, next.DisplayName, next.Status, next.BaseOfficeID, next.PositionID, nullID(next.ReportsToID), c.tenant.ID, id)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "Nomor karyawan duplikat atau referensi tidak tersedia"})
		return
	}
	if err = persistEmployeePersonal(r.Context(), tx, c.tenant.ID, id, body.employeePersonalWrite); err != nil {
		if err.Error() == "NIK_DUPLICATE" {
			writeAPIError(w, r, 409, "NIK_DUPLICATE", "NIK sudah digunakan.")
		} else {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Profil karyawan tidak dapat disimpan.")
		}
		return
	}
	if old.BaseOfficeID != next.BaseOfficeID || old.PositionID != next.PositionID || old.ReportsToID != next.ReportsToID {
		if _, err = tx.ExecContext(r.Context(), `UPDATE employee_assignments SET valid_to=UTC_TIMESTAMP(6),current_flag=NULL WHERE tenant_id=? AND employee_id=? AND current_flag=1`, c.tenant.ID, id); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Gagal menutup penempatan lama"})
			return
		}
		if _, err = tx.ExecContext(r.Context(), `UPDATE employee_assignment_history SET valid_until=UTC_TIMESTAMP(6) WHERE tenant_id=? AND employee_id=? AND valid_until IS NULL`, c.tenant.ID, id); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Gagal menutup penempatan lama"})
			return
		}
		if err = insertAssignment(r.Context(), tx, c.tenant.ID, next); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Gagal mencatat penempatan"})
			return
		}
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "update", "employees", id, old, next); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal mencatat audit"})
		return
	}
	if err = tx.Commit(); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan karyawan"})
		return
	}
	writeJSON(w, 200, next)
}

func nullID(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func insertAssignment(ctx context.Context, tx *sql.Tx, tenantID string, e employee) error {
	historyID, err := ids.New()
	if err != nil {
		return err
	}
	assignmentID, err := ids.New()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO employee_assignment_history (tenant_id,id,employee_id,base_office_id,position_id,reports_to_id,valid_from) VALUES (?,?,?,?,?,?,UTC_TIMESTAMP(6))`, tenantID, historyID, e.ID, e.BaseOfficeID, e.PositionID, nullID(e.ReportsToID))
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO employee_assignments (tenant_id,id,employee_id,region_id,office_id,division_id,department_id,position_id,reports_to_employee_id,grade,valid_from,current_flag,source_history_id)
		SELECT ?,?,?,o.region_id,o.id,d.division_id,p.department_id,p.id,?,p.grade,h.valid_from,1,?
		FROM offices o JOIN regions r ON r.tenant_id=o.tenant_id AND r.id=o.region_id JOIN positions p ON p.tenant_id=o.tenant_id AND p.id=? JOIN departments d ON d.tenant_id=p.tenant_id AND d.id=p.department_id JOIN divisions v ON v.tenant_id=d.tenant_id AND v.id=d.division_id
		JOIN employee_assignment_history h ON h.tenant_id=o.tenant_id AND h.id=?
		WHERE o.tenant_id=? AND o.id=? AND o.deleted_at IS NULL AND o.is_active=TRUE AND r.deleted_at IS NULL AND p.deleted_at IS NULL AND d.deleted_at IS NULL AND v.deleted_at IS NULL`, tenantID, assignmentID, e.ID, nullID(e.ReportsToID), historyID, e.PositionID, historyID, tenantID, e.BaseOfficeID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return errors.New("penempatan tidak dapat dipetakan")
	}
	return nil
}

func mountESS(r chi.Router, db *sql.DB) {
	service := auth.Service{DB: db}
	r.Get("/api/v1/ess/me/employee", func(w http.ResponseWriter, req *http.Request) {
		state, err := service.Current(req.Context(), cookieToken(req))
		if err != nil {
			writeAuthError(w, err)
			return
		}
		if state.SelectedTenantID == "" {
			writeJSON(w, 403, map[string]string{"error": "Pilih perusahaan terlebih dahulu"})
			return
		}
		e, err := scanEmployee(db.QueryRowContext(req.Context(), employeeSelect+` WHERE e.tenant_id=? AND e.user_id=? AND e.status IN ('active','probation','leave')`, state.SelectedTenantID, state.User.ID))
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, 403, map[string]string{"error": "Profil karyawan tidak tersedia"})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "Gagal membaca profil"})
			return
		}
		writeJSON(w, 200, e)
	})
}
