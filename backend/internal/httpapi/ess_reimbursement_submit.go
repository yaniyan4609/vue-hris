package httpapi

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"digaji.id/hris/internal/ids"
)

type reimbursementInput struct {
	CategoryID       string      `json:"category_id"`
	TrxDate          string      `json:"trx_date"`
	Amount           json.Number `json:"amount"`
	Description      string      `json:"description"`
	FileIDs          []string    `json:"file_ids"`
	ConfirmDuplicate bool        `json:"confirm_duplicate"`
}

func reimbursementDuplicateHash(tenantID, employeeID, trxDate, amount string, fileHashes []string) [32]byte {
	sort.Strings(fileHashes)
	return sha256.Sum256([]byte(strings.Join([]string{tenantID, employeeID, trxDate, amount, strings.Join(fileHashes, ",")}, "\x00")))
}

func (a adminAPI) submitEssReimbursement(w http.ResponseWriter, r *http.Request) {
	c, ok := a.receiptUploader(w, r)
	if !ok {
		return
	}
	var body reimbursementInput
	if !readJSON(w, r, &body) {
		return
	}
	date, dateErr := time.Parse("2006-01-02", body.TrxDate)
	amount, amountOK := leaveHundredths(body.Amount.String())
	body.Description = strings.TrimSpace(body.Description)
	if !fileIDPattern.MatchString(body.CategoryID) || dateErr != nil || date.Format("2006-01-02") != body.TrxDate || !amountOK || amount <= 0 || len(strings.SplitN(body.Amount.String(), ".", 2)[0]) > 16 || len([]rune(body.Description)) > 2000 || len(body.FileIDs) < 1 || len(body.FileIDs) > 5 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Kategori, tanggal, nominal, deskripsi, atau kuitansi tidak valid.")
		return
	}
	fileIDs := append([]string(nil), body.FileIDs...)
	sort.Strings(fileIDs)
	for i, id := range fileIDs {
		if !fileIDPattern.MatchString(id) || (i > 0 && id == fileIDs[i-1]) {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "ID kuitansi tidak valid atau ganda.")
			return
		}
	}
	tenantID, employeeID, userID := c.state.SelectedTenantID, c.employeeID, c.state.User.ID
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat klaim.")
		return
	}
	defer tx.Rollback()
	var locked string
	if err = tx.QueryRowContext(r.Context(), `SELECT id FROM employees WHERE tenant_id=? AND id=? FOR UPDATE`, tenantID, employeeID).Scan(&locked); err != nil {
		writeAPIError(w, r, 409, "EMPLOYEE_INVALID", "Karyawan tidak tersedia.")
		return
	}
	var categoryName string
	var limit sql.NullString
	err = tx.QueryRowContext(r.Context(), `SELECT name,CAST(monthly_limit AS CHAR) FROM reimbursement_categories WHERE tenant_id=? AND id=? AND is_active=TRUE`, tenantID, body.CategoryID).Scan(&categoryName, &limit)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "CATEGORY_NOT_FOUND", "Kategori aktif tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa kategori.")
		return
	}
	if limit.Valid {
		max, valid := leaveHundredths(limit.String)
		if !valid {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Batas kategori tidak valid.")
			return
		}
		monthStart := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, time.UTC)
		monthEnd := monthStart.AddDate(0, 1, 0)
		var usedRaw string
		if err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(CAST(SUM(amount) AS CHAR),'0.00') FROM reimbursements WHERE tenant_id=? AND employee_id=? AND category_id=? AND trx_date>=? AND trx_date<? AND status IN ('submitted','in_review','returned','approved','paid')`, tenantID, employeeID, body.CategoryID, monthStart.Format("2006-01-02"), monthEnd.Format("2006-01-02")).Scan(&usedRaw); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa batas bulanan.")
			return
		}
		used, valid := leaveHundredths(usedRaw)
		if !valid || amount > max || used > max-amount {
			writeAPIError(w, r, 409, "MONTHLY_LIMIT_EXCEEDED", "Batas bulanan kategori terlampaui.")
			return
		}
	}
	fileHashes := make([]string, 0, len(fileIDs))
	for _, fileID := range fileIDs {
		var purpose, status string
		var hash []byte
		var used int
		err = tx.QueryRowContext(r.Context(), `SELECT purpose,scan_status,sha256 FROM files WHERE tenant_id=? AND id=? AND uploaded_by=? AND deleted_at IS NULL FOR UPDATE`, tenantID, fileID, userID).Scan(&purpose, &status, &hash)
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, r, 404, "FILE_NOT_FOUND", "Kuitansi tidak ditemukan.")
			return
		}
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa kuitansi.")
			return
		}
		if purpose != "receipt" || status != "clean" {
			writeAPIError(w, r, 409, "FILE_NOT_READY", "Kuitansi belum siap dipakai.")
			return
		}
		if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM reimbursement_receipts WHERE tenant_id=? AND file_id=?`, tenantID, fileID).Scan(&used); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa pemakaian kuitansi.")
			return
		}
		if used > 0 {
			writeAPIError(w, r, 409, "FILE_ALREADY_ATTACHED", "Kuitansi sudah dipakai pada klaim lain.")
			return
		}
		fileHashes = append(fileHashes, hex.EncodeToString(hash))
	}
	duplicateHash := reimbursementDuplicateHash(tenantID, employeeID, body.TrxDate, leaveDecimal(amount), fileHashes)
	var duplicates int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM reimbursements WHERE tenant_id=? AND employee_id=? AND duplicate_hash=? AND status NOT IN ('rejected','cancelled')`, tenantID, employeeID, duplicateHash[:]).Scan(&duplicates); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa klaim serupa.")
		return
	}
	if duplicates > 0 && !body.ConfirmDuplicate {
		writeAPIError(w, r, 409, "DUPLICATE_SUSPECTED", "Klaim serupa terdeteksi; kirim confirm_duplicate untuk melanjutkan.")
		return
	}
	today := time.Now().In(c.zone).Format("2006-01-02")
	flow, err := scanFlow(tx.QueryRowContext(r.Context(), `SELECT `+flowColumns+` FROM approval_flows WHERE tenant_id=? AND request_type='reimbursement' AND is_active=TRUE AND is_default=TRUE AND valid_from<=? AND (valid_to IS NULL OR valid_to>=?) ORDER BY valid_from DESC,id LIMIT 1 FOR UPDATE`, tenantID, today, today))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 409, "FLOW_NOT_FOUND", "Alur approval reimbursement belum tersedia.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca alur approval.")
		return
	}
	step, err := scanFlowStep(tx.QueryRowContext(r.Context(), `SELECT id,level_no,approver_kind,approver_ref_id,required_permission,sla_hours FROM approval_flow_steps WHERE tenant_id=? AND flow_id=? AND level_no=1`, tenantID, flow.ID))
	if err != nil || (step.RequiredPermission != nil && *step.RequiredPermission != "reimbursement.approve") {
		writeAPIError(w, r, 409, "FLOW_INVALID", "Tahap awal approval reimbursement tidak valid.")
		return
	}
	employee, err := scanEmployee(tx.QueryRowContext(r.Context(), employeeSelect+` WHERE e.tenant_id=? AND e.id=?`, tenantID, employeeID))
	if err != nil {
		writeAPIError(w, r, 409, "EMPLOYEE_INVALID", "Penugasan karyawan belum tersedia.")
		return
	}
	target, err := a.targetFor(r.Context(), tenantID, employee)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa scope karyawan.")
		return
	}
	candidates, err := a.stepApproverCandidates(r, tenantID, today, employee, step)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menentukan approver.")
		return
	}
	eligible, err := a.eligibleApprovers(r, tenantID, c.zone.String(), employee, target, "reimbursement.approve", candidates)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa hak approver.")
		return
	}
	eligible = removeApprover(eligible, userID)
	if len(eligible) == 0 && flow.FallbackRoleID != nil {
		candidates, err = a.roleApproverCandidates(r, tenantID, *flow.FallbackRoleID, today)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menentukan fallback approver.")
			return
		}
		eligible, err = a.eligibleApprovers(r, tenantID, c.zone.String(), employee, target, "reimbursement.approve", candidates)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa hak fallback.")
			return
		}
		eligible = removeApprover(eligible, userID)
	}
	if len(eligible) == 0 {
		writeAPIError(w, r, 409, "NO_APPROVER", "Belum ada approver reimbursement yang berhak.")
		return
	}
	requestID, e1 := ids.New()
	claimID, e2 := ids.New()
	instanceID, e3 := ids.New()
	if e1 != nil || e2 != nil || e3 != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID klaim.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO requests (tenant_id,id,request_type,subject_employee_id,requested_by,ref_table,ref_id,status,submitted_at) VALUES (?,?,'reimbursement',?,?,'reimbursements',?,'in_review',UTC_TIMESTAMP(6))`, tenantID, requestID, employeeID, userID, claimID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan pengajuan.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO reimbursements (tenant_id,id,employee_id,category_id,trx_date,amount,description,duplicate_hash,status,request_id) VALUES (?,?,?,?,?,?,?,?, 'in_review',?)`, tenantID, claimID, employeeID, body.CategoryID, body.TrxDate, leaveDecimal(amount), body.Description, duplicateHash[:], requestID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan klaim.")
		return
	}
	for _, fileID := range fileIDs {
		receiptID, e := ids.New()
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat tautan kuitansi.")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO reimbursement_receipts (tenant_id,id,reimbursement_id,file_id) VALUES (?,?,?,?)`, tenantID, receiptID, claimID, fileID); err != nil {
			writeAPIError(w, r, 409, "FILE_ALREADY_ATTACHED", "Kuitansi sudah terikat pada klaim lain.")
			return
		}
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO approval_instances (tenant_id,id,request_id,flow_id,current_level,status,version) VALUES (?,?,?,?,1,'in_review',1)`, tenantID, instanceID, requestID, flow.ID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuka approval.")
		return
	}
	for _, approver := range eligible {
		taskID, e := ids.New()
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat tugas approval.")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO approval_tasks (tenant_id,id,instance_id,attempt_no,level_no,assignee_user_id,status) VALUES (?,?,?,1,1,?,'pending')`, tenantID, taskID, instanceID, approver); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat tugas approval.")
			return
		}
	}
	payload, _ := json.Marshal(map[string]any{"request_id": requestID, "request_type": "reimbursement", "approver_user_ids": eligible})
	outboxID, e := ids.New()
	if e != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat notifikasi.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO notification_outbox (tenant_id,id,event_type,aggregate_type,aggregate_id,payload_json,dedupe_key) VALUES (?,?,'approval.submitted','request',?,?,?)`, tenantID, outboxID, requestID, string(payload), "approval.submitted:"+requestID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menjadwalkan notifikasi.")
		return
	}
	if err = audit(r.Context(), tx, tenantID, userID, "submit", "requests", requestID, nil, map[string]any{"request_type": "reimbursement", "claim_id": claimID, "amount": leaveDecimal(amount), "category_id": body.CategoryID, "receipt_count": len(fileIDs), "confirmed_duplicate": body.ConfirmDuplicate}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat klaim.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan klaim.")
		return
	}
	writeAPIData(w, 201, map[string]any{"id": claimID, "request_id": requestID, "employee_id": employeeID, "category_id": body.CategoryID, "category_name": categoryName, "trx_date": body.TrxDate, "amount": fmt.Sprintf("%d.%02d", amount/100, amount%100), "status": "in_review", "receipt_count": len(fileIDs), "approval_version": 1, "approver_user_ids": eligible}, nil)
}
