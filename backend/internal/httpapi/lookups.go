package httpapi

import (
	"net/http"
	"strings"

	"digaji.id/hris/internal/access"
	"github.com/go-chi/chi/v5"
)

type lookupItem struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Badge string `json:"badge,omitempty"`
}
type lookupSource struct {
	Table      string
	Permission string
	Active     string
}

var lookupSources = map[string]lookupSource{
	"regions":                  {"regions", "organisasi.lihat", ""},
	"offices":                  {"offices", "organisasi.lihat", ""},
	"divisions":                {"divisions", "organisasi.lihat", ""},
	"departments":              {"departments", "organisasi.lihat", ""},
	"positions":                {"positions", "organisasi.lihat", ""},
	"roles":                    {"roles", "pengguna.lihat", ""},
	"leave-types":              {"leave_types", "cuti.lihat", "is_active=TRUE"},
	"shift-templates":          {"shift_templates", "jadwal.lihat", "is_active=TRUE"},
	"document-types":           {"document_types", "dokumen.lihat", ""},
	"reimbursement-categories": {"reimbursement_categories", "reimbursement.lihat", ""},
	"pay-components":           {"pay_components", "penggajian.lihat", ""},
}

func canonicalLookupPage(w http.ResponseWriter, r *http.Request) (pageOptions, bool) {
	request := r.Clone(r.Context())
	urlCopy := *r.URL
	query := urlCopy.Query()
	if size := query.Get("per_page"); size != "" {
		query.Set("pageSize", size)
	}
	urlCopy.RawQuery = query.Encode()
	request.URL = &urlCopy
	return readPage(w, request)
}

func (a adminAPI) lookupCatalog(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "type")
	if kind == "banks" {
		a.lookupBanks(w, r)
		return
	}
	source, found := lookupSources[kind]
	if !found {
		writeAPIError(w, r, 404, "NOT_FOUND", "Jenis lookup tidak ditemukan.")
		return
	}
	var tenantID string
	if kind == "leave-types" {
		state, err := a.auth.Current(r.Context(), cookieToken(r))
		if err != nil {
			writeAuthError(w, err)
			return
		}
		for _, tenant := range state.Tenants {
			if tenant.ID == state.SelectedTenantID && tenant.ESS {
				_, decision, err := a.access.Decide(r.Context(), tenant.ID, state.User.ID, tenant.Timezone, "cuti.buat")
				if err != nil {
					writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa akses.")
					return
				}
				if decision.Allowed {
					tenantID = tenant.ID
				}
				break
			}
		}
	}
	if tenantID == "" {
		c, ok := a.authorize(w, r, source.Permission)
		if !ok {
			return
		}
		tenantID = c.tenant.ID
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	q, ok := searchTerm(w, r)
	if !ok {
		return
	}
	where := "tenant_id=?"
	args := []any{tenantID}
	if source.Active != "" {
		where += " AND " + source.Active
	}
	if q != "" {
		where += " AND (LOCATE(LOWER(?),LOWER(name))>0 OR LOCATE(LOWER(?),LOWER(code))>0)"
		args = append(args, q, q)
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM "+source.Table+" WHERE "+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca lookup.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), "SELECT id,name,code FROM "+source.Table+" WHERE "+where+" ORDER BY name,id LIMIT ? OFFSET ?", append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca lookup.")
		return
	}
	defer rows.Close()
	items := []lookupItem{}
	for rows.Next() {
		var item lookupItem
		if err := rows.Scan(&item.ID, &item.Label, &item.Badge); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca lookup.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca lookup.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}

var employeeLookupPermissions = map[string]string{
	"broadcast": "broadcast.buat",
	"roster":    "jadwal.buat",
	"approval":  "persetujuan.lihat",
	"leave":     "cuti.buat",
	"contract":  "kontrak.buat",
	"document":  "dokumen.buat",
}

func (a adminAPI) lookupEmployees(w http.ResponseWriter, r *http.Request) {
	purpose := r.URL.Query().Get("for")
	code := employeeLookupPermissions[purpose]
	if code == "" {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Parameter for tidak valid atau belum diisi.")
		return
	}
	c, ok := a.authorize(w, r, code)
	if !ok {
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	q, ok := searchTerm(w, r)
	if !ok {
		return
	}
	filter, filterArgs := access.EmployeeWhere(c.subject, c.effective)
	where := "e.tenant_id=? AND " + filter + " AND e.status IN ('active','probation','leave')"
	args := append([]any{c.tenant.ID}, filterArgs...)
	if q != "" {
		where += " AND (LOCATE(LOWER(?),LOWER(e.display_name))>0 OR LOCATE(LOWER(?),LOWER(e.employee_number))>0)"
		args = append(args, q, q)
	}
	join := ` FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE`
	var total int
	if err := a.db.QueryRowContext(r.Context(), "SELECT COUNT(*)"+join+" WHERE "+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca karyawan.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), "SELECT e.id,e.display_name,e.employee_number"+join+" WHERE "+where+" ORDER BY e.display_name,e.id LIMIT ? OFFSET ?", append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca karyawan.")
		return
	}
	defer rows.Close()
	items := []lookupItem{}
	for rows.Next() {
		var item lookupItem
		if err := rows.Scan(&item.ID, &item.Label, &item.Badge); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca karyawan.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca karyawan.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}

func (a adminAPI) lookupBanks(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "karyawan.lihat_sensitif")
	if !ok {
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	q, ok := searchTerm(w, r)
	if !ok {
		return
	}
	filter, filterArgs := access.EmployeeWhere(c.subject, c.effective)
	join := ` FROM employee_bank_accounts b JOIN employees e ON e.tenant_id=b.tenant_id AND e.id=b.employee_id JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE`
	where := "b.tenant_id=? AND " + filter
	args := append([]any{c.tenant.ID}, filterArgs...)
	if q != "" {
		where += " AND LOCATE(LOWER(?),LOWER(b.bank_code))>0"
		args = append(args, q)
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), "SELECT COUNT(DISTINCT b.bank_code)"+join+" WHERE "+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca bank.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), "SELECT DISTINCT b.bank_code"+join+" WHERE "+where+" ORDER BY b.bank_code LIMIT ? OFFSET ?", append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca bank.")
		return
	}
	defer rows.Close()
	items := []lookupItem{}
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca bank.")
			return
		}
		items = append(items, lookupItem{ID: code, Label: strings.ToUpper(code)})
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca bank.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}
