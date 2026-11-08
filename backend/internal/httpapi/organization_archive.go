package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
)

var organizationAssignmentColumn = map[string]string{
	"regions": "region_id", "offices": "office_id", "divisions": "division_id",
	"departments": "department_id", "positions": "position_id",
}

var organizationChildren = map[string]struct{ table, column string }{
	"regions": {"offices", "region_id"}, "divisions": {"departments", "division_id"},
	"departments": {"positions", "department_id"},
}

var organizationParentTable = map[string]string{
	"region_id": "regions", "division_id": "divisions", "department_id": "departments",
}

func activeOrganizationParent(ctx context.Context, tx *sql.Tx, tenantID string, kind orgKind, parentID string) (bool, error) {
	if kind.parentColumn == "" {
		return true, nil
	}
	var id string
	err := tx.QueryRowContext(ctx, "SELECT id FROM "+organizationParentTable[kind.parentColumn]+" WHERE tenant_id=? AND id=? AND deleted_at IS NULL FOR UPDATE", tenantID, parentID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (a adminAPI) archiveOrganization(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, ok := a.authorize(w, r, "organisasi.hapus")
		if !ok {
			return
		}
		config, known := orgKinds[kind]
		id := chi.URLParam(r, "id")
		if !known || !fileIDPattern.MatchString(id) {
			writeAPIError(w, r, 404, "NOT_FOUND", "Data organisasi tidak ditemukan.")
			return
		}
		tx, err := a.db.BeginTx(r.Context(), nil)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengarsipkan data organisasi.")
			return
		}
		defer tx.Rollback()
		var code, name string
		var deleted sql.NullTime
		err = tx.QueryRowContext(r.Context(), "SELECT code,name,deleted_at FROM "+config.table+" WHERE tenant_id=? AND id=? FOR UPDATE", c.tenant.ID, id).Scan(&code, &name, &deleted)
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, r, 404, "NOT_FOUND", "Data organisasi tidak ditemukan.")
			return
		}
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca data organisasi.")
			return
		}
		if deleted.Valid {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		inUse := func(query string) (bool, error) {
			var count int
			e := tx.QueryRowContext(r.Context(), query, c.tenant.ID, id).Scan(&count)
			return count != 0, e
		}
		if child, exists := organizationChildren[kind]; exists {
			used, e := inUse("SELECT COUNT(*) FROM " + child.table + " WHERE tenant_id=? AND " + child.column + "=? AND deleted_at IS NULL")
			if e != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa data turunan.")
				return
			}
			if used {
				writeAPIError(w, r, 409, "IN_USE", "Data organisasi masih memiliki data turunan aktif.")
				return
			}
		}
		used, e := inUse("SELECT COUNT(*) FROM employee_assignments WHERE tenant_id=? AND " + organizationAssignmentColumn[kind] + "=? AND current_flag=1")
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa penugasan aktif.")
			return
		}
		if used {
			writeAPIError(w, r, 409, "IN_USE", "Data organisasi masih dipakai penugasan aktif.")
			return
		}
		if kind == "offices" || kind == "positions" {
			column := "base_office_id"
			if kind == "positions" {
				column = "position_id"
			}
			used, e = inUse("SELECT COUNT(*) FROM employees WHERE tenant_id=? AND " + column + "=? AND deleted_at IS NULL AND status IN ('active','probation','leave')")
			if e != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa karyawan aktif.")
				return
			}
			if used {
				writeAPIError(w, r, 409, "IN_USE", "Data organisasi masih dipakai karyawan aktif.")
				return
			}
		}
		sets := "deleted_at=UTC_TIMESTAMP(6)"
		if kind == "offices" {
			sets += ",is_active=FALSE"
		}
		if _, err = tx.ExecContext(r.Context(), "UPDATE "+config.table+" SET "+sets+" WHERE tenant_id=? AND id=?", c.tenant.ID, id); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengarsipkan data organisasi.")
			return
		}
		if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "archive", config.table, id, map[string]any{"code": code, "name": name, "status": "active"}, map[string]any{"code": code, "name": name, "status": "archived"}); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat arsip organisasi.")
			return
		}
		if err = tx.Commit(); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengarsipkan data organisasi.")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
