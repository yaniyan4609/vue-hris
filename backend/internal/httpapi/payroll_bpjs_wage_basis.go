package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"digaji.id/hris/internal/ids"
	"github.com/go-sql-driver/mysql"
)

type bpjsWageProgram struct {
	Program      string   `json:"program"`
	ComponentIDs []string `json:"component_ids"`
}

type bpjsOfficeMinimum struct {
	OfficeID string      `json:"office_id"`
	Amount   json.Number `json:"amount"`
}

type bpjsWageBasisData struct {
	Programs       []bpjsWageProgram   `json:"programs"`
	OfficeMinimums []bpjsOfficeMinimum `json:"office_minimums"`
}

type bpjsWageBasisView struct {
	ID              string  `json:"id"`
	EffectiveFrom   string  `json:"effective_from"`
	EffectiveTo     *string `json:"effective_to"`
	SourceReference string  `json:"source_reference"`
	bpjsWageBasisData
}

func scanBPJSWageBasis(row interface{ Scan(...any) error }) (bpjsWageBasisView, error) {
	var item bpjsWageBasisView
	var from time.Time
	var to sql.NullTime
	var raw []byte
	err := row.Scan(&item.ID, &from, &to, &raw, &item.SourceReference)
	if err != nil {
		return item, err
	}
	item.EffectiveFrom = from.Format("2006-01-02")
	if to.Valid {
		value := to.Time.Format("2006-01-02")
		item.EffectiveTo = &value
	}
	err = json.Unmarshal(raw, &item.bpjsWageBasisData)
	return item, err
}

func (a adminAPI) getBPJSWageBasis(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "penggajian.lihat")
	if !ok {
		return
	}
	date, ok := payrollRuleDate(w, r, c.tenant.Timezone)
	if !ok {
		return
	}
	item, err := scanBPJSWageBasis(a.db.QueryRowContext(r.Context(), `SELECT id,effective_from,effective_to,data_json,source_reference FROM bpjs_wage_basis_policies WHERE tenant_id=? AND effective_from<=? AND (effective_to IS NULL OR effective_to>=?) ORDER BY effective_from DESC LIMIT 1`, c.tenant.ID, date, date))
	if err == nil {
		writeAPIData(w, 200, map[string]any{"date": date, "configured": true, "policy": item}, nil)
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca dasar upah BPJS.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT id,code FROM pay_components WHERE tenant_id=? AND is_active=TRUE AND component_type='earning' AND calc_type='fixed' ORDER BY code`, c.tenant.ID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca komponen dasar upah.")
		return
	}
	suggested := []string{}
	for rows.Next() {
		var id, code string
		if err = rows.Scan(&id, &code); err != nil {
			break
		}
		if code == "GAJI_POKOK" || code == "TUNJANGAN_TETAP" || strings.HasPrefix(code, "TUNJANGAN_TETAP_") {
			suggested = append(suggested, id)
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca komponen dasar upah.")
		return
	}
	suggestedPrograms := make([]bpjsWageProgram, 0, len(requiredPayrollBPJSPrograms))
	for _, program := range requiredPayrollBPJSPrograms {
		suggestedPrograms = append(suggestedPrograms, bpjsWageProgram{Program: program, ComponentIDs: suggested})
	}
	writeAPIData(w, 200, map[string]any{"date": date, "configured": false, "policy": nil, "suggested_component_ids": suggested, "suggested_programs": suggestedPrograms}, nil)
}

func validateBPJSWageBasis(data bpjsWageBasisData, eligibleComponents, activeOffices map[string]bool) bool {
	if len(data.Programs) != len(requiredPayrollBPJSPrograms) || len(data.OfficeMinimums) != len(activeOffices) {
		return false
	}
	seenPrograms := map[string]bool{}
	for _, program := range data.Programs {
		validProgram := false
		for _, required := range requiredPayrollBPJSPrograms {
			validProgram = validProgram || program.Program == required
		}
		if !validProgram || seenPrograms[program.Program] || len(program.ComponentIDs) == 0 || len(program.ComponentIDs) > 100 {
			return false
		}
		seenPrograms[program.Program] = true
		seenComponents := map[string]bool{}
		for _, id := range program.ComponentIDs {
			if !eligibleComponents[id] || seenComponents[id] {
				return false
			}
			seenComponents[id] = true
		}
	}
	seenOffices := map[string]bool{}
	for _, item := range data.OfficeMinimums {
		amount, ok := validPayrollMoney(item.Amount.String())
		if !activeOffices[item.OfficeID] || seenOffices[item.OfficeID] || !ok || amount <= 0 {
			return false
		}
		seenOffices[item.OfficeID] = true
	}
	return true
}

func (a adminAPI) publishBPJSWageBasis(w http.ResponseWriter, r *http.Request) {
	c, ok := a.payrollRuleWriter(w, r)
	if !ok {
		return
	}
	var input struct {
		EffectiveFrom   string `json:"effective_from"`
		SourceReference string `json:"source_reference"`
		bpjsWageBasisData
	}
	if !readJSON(w, r, &input) {
		return
	}
	input.SourceReference = strings.TrimSpace(input.SourceReference)
	if !validRuleDates(input.EffectiveFrom, nil) || input.SourceReference == "" || len([]rune(input.SourceReference)) > 500 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal berlaku atau sumber dasar upah BPJS tidak valid.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan dasar upah BPJS.")
		return
	}
	defer tx.Rollback()
	if !lockPayrollRuleTenant(w, r, tx, c.tenant.ID, input.EffectiveFrom) {
		return
	}
	eligible := map[string]bool{}
	rows, err := tx.QueryContext(r.Context(), `SELECT id FROM pay_components WHERE tenant_id=? AND is_active=TRUE AND component_type='earning' AND calc_type='fixed'`, c.tenant.ID)
	if err == nil {
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				break
			}
			eligible[id] = true
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa komponen gaji.")
		return
	}
	offices := map[string]bool{}
	rows, err = tx.QueryContext(r.Context(), `SELECT id FROM offices WHERE tenant_id=? AND is_active=TRUE AND deleted_at IS NULL`, c.tenant.ID)
	if err == nil {
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				break
			}
			offices[id] = true
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa kantor.")
		return
	}
	if !validateBPJSWageBasis(input.bpjsWageBasisData, eligible, offices) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Pilih komponen fixed aktif untuk lima program dan isi upah minimum setiap kantor aktif.")
		return
	}
	var previousID, previousFrom string
	var previousTo sql.NullTime
	err = tx.QueryRowContext(r.Context(), `SELECT id,DATE_FORMAT(effective_from,'%Y-%m-%d'),effective_to FROM bpjs_wage_basis_policies WHERE tenant_id=? ORDER BY effective_from DESC LIMIT 1 FOR UPDATE`, c.tenant.ID).Scan(&previousID, &previousFrom, &previousTo)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca versi dasar upah sebelumnya.")
		return
	}
	if err == nil {
		if input.EffectiveFrom <= previousFrom {
			writeAPIError(w, r, 409, "RULE_VERSION_CONFLICT", "Tanggal berlaku harus setelah versi terbaru.")
			return
		}
		if !previousTo.Valid || previousTo.Time.Format("2006-01-02") >= input.EffectiveFrom {
			if _, err = tx.ExecContext(r.Context(), `UPDATE bpjs_wage_basis_policies SET effective_to=? WHERE tenant_id=? AND id=?`, dateBefore(input.EffectiveFrom), c.tenant.ID, previousID); err != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menutup versi dasar upah sebelumnya.")
				return
			}
		}
	}
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID dasar upah.")
		return
	}
	raw, err := json.Marshal(input.bpjsWageBasisData)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan dasar upah BPJS.")
		return
	}
	_, err = tx.ExecContext(r.Context(), `INSERT INTO bpjs_wage_basis_policies (tenant_id,id,effective_from,data_json,source_reference,created_by) VALUES (?,?,?,?,?,?)`, c.tenant.ID, id, input.EffectiveFrom, raw, input.SourceReference, c.state.User.ID)
	if err != nil {
		var dbError *mysql.MySQLError
		if errors.As(err, &dbError) && dbError.Number == 1062 {
			writeAPIError(w, r, 409, "RULE_VERSION_CONFLICT", "Versi dasar upah sudah ada.")
		} else {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan dasar upah BPJS.")
		}
		return
	}
	view := bpjsWageBasisView{ID: id, EffectiveFrom: input.EffectiveFrom, SourceReference: input.SourceReference, bpjsWageBasisData: input.bpjsWageBasisData}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "publish", "bpjs_wage_basis_policies", id, map[string]any{"previous_id": previousID}, view); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat dasar upah BPJS.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan dasar upah BPJS.")
		return
	}
	writeAPIData(w, 201, view, nil)
}
