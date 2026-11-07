package httpapi

import (
	"database/sql"
	"net/http"
	"time"
)

type employeeAssignment struct {
	ID                  string     `json:"id"`
	EmployeeID          string     `json:"employee_id"`
	RegionID            string     `json:"region_id"`
	OfficeID            string     `json:"office_id"`
	DivisionID          string     `json:"division_id"`
	DepartmentID        string     `json:"department_id"`
	PositionID          string     `json:"position_id"`
	ReportsToEmployeeID *string    `json:"reports_to_employee_id"`
	Grade               *string    `json:"grade"`
	ValidFrom           time.Time  `json:"valid_from"`
	ValidTo             *time.Time `json:"valid_to"`
	Current             bool       `json:"current"`
}

func scanAssignment(row interface{ Scan(...any) error }) (employeeAssignment, error) {
	var item employeeAssignment
	var manager, grade sql.NullString
	var validTo sql.NullTime
	var current sql.NullInt64
	err := row.Scan(&item.ID, &item.EmployeeID, &item.RegionID, &item.OfficeID, &item.DivisionID, &item.DepartmentID, &item.PositionID, &manager, &grade, &item.ValidFrom, &validTo, &current)
	if err != nil {
		return item, err
	}
	if manager.Valid {
		item.ReportsToEmployeeID = &manager.String
	}
	if grade.Valid {
		item.Grade = &grade.String
	}
	if validTo.Valid {
		item.ValidTo = &validTo.Time
	}
	item.Current = current.Valid && current.Int64 == 1
	return item, nil
}

func (a adminAPI) listEmployeeAssignments(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "karyawan.lihat")
	if !ok {
		return
	}
	employeeID, ok := a.scopedEmployee(w, r, c)
	if !ok {
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT id,employee_id,region_id,office_id,division_id,department_id,position_id,reports_to_employee_id,grade,valid_from,valid_to,current_flag FROM employee_assignments WHERE tenant_id=? AND employee_id=? ORDER BY valid_from DESC,id DESC`, c.tenant.ID, employeeID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca riwayat penugasan.")
		return
	}
	defer rows.Close()
	items := []employeeAssignment{}
	for rows.Next() {
		item, err := scanAssignment(rows)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca riwayat penugasan.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca riwayat penugasan.")
		return
	}
	writeAPIData(w, 200, items, nil)
}
