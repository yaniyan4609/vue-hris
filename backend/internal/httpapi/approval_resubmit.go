package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

func (a adminAPI) resubmitEssRequest(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	requestID := chi.URLParam(r, "requestId")
	if len(requestID) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengajuan tidak ditemukan.")
		return
	}
	var body struct {
		Version    uint64          `json:"version"`
		Correction json.RawMessage `json:"correction"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	var correction map[string]any
	if body.Version == 0 || len(body.Correction) > 8192 || json.Unmarshal(body.Correction, &correction) != nil || len(correction) < 1 || len(correction) > 7 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Versi dan catatan perbaikan pengajuan wajib diisi.")
		return
	}
	note, noteOK := correction["note"].(string)
	note = strings.TrimSpace(note)
	if !noteOK || note == "" || len([]rune(note)) > 2000 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Catatan perbaikan tidak valid.")
		return
	}
	normalized := map[string]any{"note": note}
	for key, value := range correction {
		switch key {
		case "note":
		case "proposed_minutes":
			minutes, ok := value.(float64)
			if !ok || minutes < 1 || minutes > 1440 || minutes != float64(int(minutes)) {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Menit lembur perbaikan tidak valid.")
				return
			}
			normalized[key] = int(minutes)
		case "reason":
			reason, ok := value.(string)
			reason = strings.TrimSpace(reason)
			if !ok || reason == "" || len([]rune(reason)) > 2000 {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Alasan lembur perbaikan tidak valid.")
				return
			}
			normalized[key] = reason
		case "start_date", "end_date", "leave_type_id", "attachment_file_id":
			if value == nil && key == "attachment_file_id" {
				normalized[key] = nil
				break
			}
			text, ok := value.(string)
			if !ok || text == "" || len(text) > 26 {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Field cuti perbaikan tidak valid.")
				return
			}
			normalized[key] = text
		case "requested_type", "requested_time", "original_event_id":
			if key == "original_event_id" && value == nil {
				normalized[key] = nil
				break
			}
			text, ok := value.(string)
			if !ok || text == "" || len(text) > 64 {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Field koreksi presensi tidak valid.")
				return
			}
			normalized[key] = text
		default:
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Field perbaikan tidak dikenal.")
			return
		}
	}
	correctionJSON, _ := json.Marshal(normalized)
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengirim ulang pengajuan.")
		return
	}
	defer tx.Rollback()
	var kind, requester, status string
	var subjectID, refTable, refID sql.NullString
	var attemptNo, version uint64
	err = tx.QueryRowContext(r.Context(), `SELECT request_type,subject_employee_id,requested_by,ref_table,ref_id,status,attempt_no,version FROM requests WHERE tenant_id=? AND id=? FOR UPDATE`, c.state.SelectedTenantID, requestID).Scan(&kind, &subjectID, &requester, &refTable, &refID, &status, &attemptNo, &version)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (requester != c.state.User.ID || !subjectID.Valid || subjectID.String != c.employeeID)) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengajuan tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengajuan.")
		return
	}
	if status != "returned" {
		writeAPIError(w, r, 409, "REQUEST_NOT_RETURNED", "Hanya pengajuan yang dikembalikan dapat dikirim ulang.")
		return
	}
	if version != body.Version {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Pengajuan sudah berubah.")
		return
	}
	if approvalPermissions[kind] == "" || attemptNo >= 100 {
		writeAPIError(w, r, 409, "REQUEST_INVALID", "Jenis atau jumlah kirim ulang pengajuan tidak valid.")
		return
	}
	if kind != "overtime" && kind != "leave" && kind != "attendance_correction" && len(normalized) > 1 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Perbaikan field domain belum tersedia untuk jenis pengajuan ini.")
		return
	}
	if kind != "overtime" && kind != "leave" && kind != "reimbursement" && kind != "loan" && kind != "shift_swap" && kind != "attendance_correction" && refTable.Valid {
		writeAPIError(w, r, 409, "ACTION_NOT_READY", "Kirim ulang jenis pengajuan ini menunggu handler domain.")
		return
	}
	if kind == "overtime" && (!refTable.Valid || refTable.String != "overtime_requests" || !refID.Valid) {
		writeAPIError(w, r, 409, "REQUEST_INVALID", "Referensi data lembur tidak valid.")
		return
	}
	if kind == "leave" && (!refTable.Valid || refTable.String != "leave_requests" || !refID.Valid) {
		writeAPIError(w, r, 409, "REQUEST_INVALID", "Referensi data cuti tidak valid.")
		return
	}
	if kind == "reimbursement" && (!refTable.Valid || refTable.String != "reimbursements" || !refID.Valid) {
		writeAPIError(w, r, 409, "REQUEST_INVALID", "Referensi klaim tidak valid.")
		return
	}
	if kind == "loan" && (!refTable.Valid || refTable.String != "employee_loans" || !refID.Valid) {
		writeAPIError(w, r, 409, "REQUEST_INVALID", "Referensi kasbon tidak valid.")
		return
	}
	if kind == "shift_swap" && (!refTable.Valid || refTable.String != "shift_swaps" || !refID.Valid) {
		writeAPIError(w, r, 409, "REQUEST_INVALID", "Referensi pertukaran shift tidak valid.")
		return
	}
	if kind == "attendance_correction" && (!refTable.Valid || refTable.String != "attendance_corrections" || !refID.Valid) {
		writeAPIError(w, r, 409, "REQUEST_INVALID", "Referensi koreksi tidak valid.")
		return
	}
	if kind == "attendance_correction" {
		for key := range normalized {
			switch key {
			case "note", "reason", "requested_type", "requested_time", "original_event_id", "attachment_file_id":
			default:
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Field perbaikan tidak berlaku untuk koreksi presensi.")
				return
			}
		}
	} else if _, has := normalized["requested_time"]; has {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Field koreksi presensi tidak berlaku untuk pengajuan ini.")
		return
	} else if _, has := normalized["requested_type"]; has {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Field koreksi presensi tidak berlaku untuk pengajuan ini.")
		return
	} else if _, has := normalized["original_event_id"]; has {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Field koreksi presensi tidak berlaku untuk pengajuan ini.")
		return
	}
	if kind == "overtime" {
		for _, key := range []string{"start_date", "end_date", "leave_type_id", "attachment_file_id"} {
			if _, exists := normalized[key]; exists {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Field cuti tidak berlaku untuk lembur.")
				return
			}
		}
	}
	if kind == "leave" {
		if _, exists := normalized["proposed_minutes"]; exists {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Menit lembur tidak berlaku untuk cuti.")
			return
		}
	}
	var instanceID, flowID, instanceStatus string
	var instanceAttempt, instanceVersion uint64
	err = tx.QueryRowContext(r.Context(), `SELECT id,flow_id,status,attempt_no,version FROM approval_instances WHERE tenant_id=? AND request_id=? FOR UPDATE`, c.state.SelectedTenantID, requestID).Scan(&instanceID, &flowID, &instanceStatus, &instanceAttempt, &instanceVersion)
	if err != nil || instanceStatus != "returned" || instanceAttempt != attemptNo {
		writeAPIError(w, r, 409, "REQUEST_INVALID", "Riwayat proses persetujuan tidak sesuai.")
		return
	}
	var fallback sql.NullString
	if err := tx.QueryRowContext(r.Context(), `SELECT fallback_role_id FROM approval_flows WHERE tenant_id=? AND id=?`, c.state.SelectedTenantID, flowID).Scan(&fallback); err != nil {
		writeAPIError(w, r, 409, "FLOW_NOT_FOUND", "Alur persetujuan awal tidak tersedia.")
		return
	}
	step, err := scanFlowStep(tx.QueryRowContext(r.Context(), `SELECT id,level_no,approver_kind,approver_ref_id,required_permission,sla_hours FROM approval_flow_steps WHERE tenant_id=? AND flow_id=? AND level_no=1`, c.state.SelectedTenantID, flowID))
	if err != nil || (step.RequiredPermission != nil && *step.RequiredPermission != approvalPermissions[kind]) {
		writeAPIError(w, r, 409, "FLOW_INVALID", "Tahap awal alur persetujuan tidak valid.")
		return
	}
	employee, err := scanEmployee(tx.QueryRowContext(r.Context(), employeeSelect+` WHERE e.tenant_id=? AND e.id=?`, c.state.SelectedTenantID, subjectID.String))
	if err != nil {
		writeAPIError(w, r, 409, "REQUEST_INVALID", "Profil karyawan tidak tersedia.")
		return
	}
	target, err := a.targetFor(r.Context(), c.state.SelectedTenantID, employee)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca scope karyawan.")
		return
	}
	today := time.Now().In(c.zone).Format("2006-01-02")
	candidates, err := a.stepApproverCandidates(r, c.state.SelectedTenantID, today, employee, step)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menentukan approver.")
		return
	}
	eligible, err := a.eligibleApprovers(r, c.state.SelectedTenantID, c.zone.String(), employee, target, approvalPermissions[kind], candidates, kind)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa hak approver.")
		return
	}
	if len(eligible) == 0 && fallback.Valid {
		candidates, err = a.roleApproverCandidates(r, c.state.SelectedTenantID, fallback.String, today)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menentukan fallback approver.")
			return
		}
		eligible, err = a.eligibleApprovers(r, c.state.SelectedTenantID, c.zone.String(), employee, target, approvalPermissions[kind], candidates, kind)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa hak fallback.")
			return
		}
	}
	if len(eligible) == 0 {
		writeAPIError(w, r, 409, "NO_APPROVER", "Belum ada approver yang berhak untuk tahap awal.")
		return
	}
	if kind == "overtime" {
		var proposed int
		var workDate time.Time
		var overtimeReason sql.NullString
		var overtimeStatus string
		if err := tx.QueryRowContext(r.Context(), `SELECT work_date,proposed_minutes,reason,status FROM overtime_requests WHERE tenant_id=? AND id=? AND request_id=? AND employee_id=? FOR UPDATE`, c.state.SelectedTenantID, refID.String, requestID, subjectID.String).Scan(&workDate, &proposed, &overtimeReason, &overtimeStatus); err != nil || overtimeStatus != "returned" {
			writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Status data lembur tidak sesuai pengajuan.")
			return
		}
		var locked string
		if err := tx.QueryRowContext(r.Context(), `SELECT id FROM employees WHERE tenant_id=? AND id=? FOR UPDATE`, c.state.SelectedTenantID, subjectID.String).Scan(&locked); err != nil {
			writeAPIError(w, r, 409, "EMPLOYEE_INVALID", "Karyawan tidak tersedia.")
			return
		}
		var duplicate int
		if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM overtime_requests WHERE tenant_id=? AND employee_id=? AND work_date=? AND id<>? AND status IN ('submitted','in_review','approved')`, c.state.SelectedTenantID, subjectID.String, workDate.Format("2006-01-02"), refID.String).Scan(&duplicate); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa bentrok lembur.")
			return
		}
		if duplicate > 0 {
			writeAPIError(w, r, 409, "OVERTIME_EXISTS", "Sudah ada pengajuan lembur aktif pada tanggal itu.")
			return
		}
		if value, ok := normalized["proposed_minutes"]; ok {
			proposed = value.(int)
		}
		reason := overtimeReason.String
		if value, ok := normalized["reason"]; ok {
			reason = value.(string)
		}
		result, err := tx.ExecContext(r.Context(), `UPDATE overtime_requests SET proposed_minutes=?,reason=?,status='in_review' WHERE tenant_id=? AND id=? AND request_id=? AND employee_id=? AND status='returned'`, proposed, reason, c.state.SelectedTenantID, refID.String, requestID, subjectID.String)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuka ulang data lembur.")
			return
		}
		rows, _ := result.RowsAffected()
		if rows != 1 {
			writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Status data lembur tidak sesuai pengajuan.")
			return
		}
	}
	if kind == "leave" {
		var oldType, oldStatus string
		var oldStart, oldEnd time.Time
		var oldReason, oldAttachment sql.NullString
		if err := tx.QueryRowContext(r.Context(), `SELECT leave_type_id,start_date,end_date,reason,attachment_file_id,status FROM leave_requests WHERE tenant_id=? AND id=? AND request_id=? AND employee_id=? FOR UPDATE`, c.state.SelectedTenantID, refID.String, requestID, subjectID.String).Scan(&oldType, &oldStart, &oldEnd, &oldReason, &oldAttachment, &oldStatus); err != nil || oldStatus != "returned" {
			writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Status data cuti tidak sesuai pengajuan.")
			return
		}
		in := leaveInput{LeaveTypeID: oldType, StartDate: oldStart.Format("2006-01-02"), EndDate: oldEnd.Format("2006-01-02"), Reason: oldReason.String}
		if oldAttachment.Valid {
			in.AttachmentFileID = &oldAttachment.String
		}
		if value, ok := normalized["leave_type_id"]; ok {
			in.LeaveTypeID = value.(string)
		}
		if value, ok := normalized["start_date"]; ok {
			in.StartDate = value.(string)
		}
		if value, ok := normalized["end_date"]; ok {
			in.EndDate = value.(string)
		}
		if value, ok := normalized["reason"]; ok {
			in.Reason = value.(string)
		}
		if value, ok := normalized["attachment_file_id"]; ok {
			in.AttachmentFileID = nil
			if value != nil {
				v := value.(string)
				in.AttachmentFileID = &v
			}
		}
		var locked string
		if err := tx.QueryRowContext(r.Context(), `SELECT id FROM employees WHERE tenant_id=? AND id=? FOR UPDATE`, c.state.SelectedTenantID, subjectID.String).Scan(&locked); err != nil {
			writeAPIError(w, r, 409, "EMPLOYEE_INVALID", "Karyawan tidak tersedia.")
			return
		}
		prepared, err := a.prepareLeave(r, tx, c.state.SelectedTenantID, subjectID.String, c.zone.String(), in)
		if err != nil {
			writeLeaveIssue(w, r, err)
			return
		}
		if prepared.Flow.ID != flowID {
			writeAPIError(w, r, 409, "FLOW_CHANGED", "Alur approval cuti telah berubah. Hubungi admin.")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `DELETE FROM leave_request_allocations WHERE tenant_id=? AND leave_request_id=?`, c.state.SelectedTenantID, refID.String); err != nil {
			writeLeaveIssue(w, r, err)
			return
		}
		for _, allocation := range prepared.Allocations {
			allocationID, e := ids.New()
			if e != nil {
				writeLeaveIssue(w, r, e)
				return
			}
			if _, e = tx.ExecContext(r.Context(), `INSERT INTO leave_request_allocations (tenant_id,id,leave_request_id,period_year,first_work_date,days) VALUES (?,?,?,?,?,?)`, c.state.SelectedTenantID, allocationID, refID.String, allocation.Year, allocation.FirstWorkDate, allocation.Days); e != nil {
				writeLeaveIssue(w, r, e)
				return
			}
		}
		result, err := tx.ExecContext(r.Context(), `UPDATE leave_requests SET leave_type_id=?,start_date=?,end_date=?,days=?,reason=?,attachment_file_id=?,status='in_review' WHERE tenant_id=? AND id=? AND request_id=? AND employee_id=? AND status='returned'`, in.LeaveTypeID, in.StartDate, in.EndDate, prepared.Days, in.Reason, in.AttachmentFileID, c.state.SelectedTenantID, refID.String, requestID, subjectID.String)
		if err != nil {
			writeLeaveIssue(w, r, err)
			return
		}
		affected, _ := result.RowsAffected()
		if affected != 1 {
			writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Status data cuti tidak sesuai pengajuan.")
			return
		}
	}
	if kind == "reimbursement" {
		var status string
		if err := tx.QueryRowContext(r.Context(), `SELECT status FROM reimbursements WHERE tenant_id=? AND id=? AND request_id=? AND employee_id=? FOR UPDATE`, c.state.SelectedTenantID, refID.String, requestID, subjectID.String).Scan(&status); err != nil || status != "returned" {
			writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Status klaim tidak sesuai pengajuan.")
			return
		}
		var receipts int
		if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM reimbursement_receipts rr JOIN files f ON f.tenant_id=rr.tenant_id AND f.id=rr.file_id WHERE rr.tenant_id=? AND rr.reimbursement_id=? AND f.scan_status='clean' AND f.deleted_at IS NULL`, c.state.SelectedTenantID, refID.String).Scan(&receipts); err != nil || receipts == 0 {
			writeAPIError(w, r, 409, "RECEIPT_REQUIRED", "Kuitansi terverifikasi wajib tersedia sebelum kirim ulang.")
			return
		}
		result, err := tx.ExecContext(r.Context(), `UPDATE reimbursements SET status='in_review' WHERE tenant_id=? AND id=? AND request_id=? AND employee_id=? AND status='returned'`, c.state.SelectedTenantID, refID.String, requestID, subjectID.String)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuka ulang klaim.")
			return
		}
		count, _ := result.RowsAffected()
		if count != 1 {
			writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Status klaim telah berubah.")
			return
		}
	}
	if kind == "loan" {
		var locked string
		if err := tx.QueryRowContext(r.Context(), `SELECT id FROM employees WHERE tenant_id=? AND id=? FOR UPDATE`, c.state.SelectedTenantID, subjectID.String).Scan(&locked); err != nil {
			writeAPIError(w, r, 409, "EMPLOYEE_INVALID", "Karyawan tidak tersedia.")
			return
		}
		var status, amountRaw string
		var tenor int
		if err := tx.QueryRowContext(r.Context(), `SELECT status,CAST(amount AS CHAR),tenor_months FROM employee_loans WHERE tenant_id=? AND id=? AND request_id=? AND employee_id=? FOR UPDATE`, c.state.SelectedTenantID, refID.String, requestID, subjectID.String).Scan(&status, &amountRaw, &tenor); err != nil || status != "returned" {
			writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Status kasbon tidak sesuai pengajuan.")
			return
		}
		amount, valid := leaveHundredths(amountRaw)
		if !valid {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Nominal kasbon tidak valid.")
			return
		}
		if _, err := calculateLoan(r.Context(), tx, c.state.SelectedTenantID, subjectID.String, amount, tenor); err != nil {
			writeLoanIssue(w, r, err)
			return
		}
		result, err := tx.ExecContext(r.Context(), `UPDATE employee_loans SET status='in_review' WHERE tenant_id=? AND id=? AND request_id=? AND employee_id=? AND status='returned'`, c.state.SelectedTenantID, refID.String, requestID, subjectID.String)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuka ulang kasbon.")
			return
		}
		count, _ := result.RowsAffected()
		if count != 1 {
			writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Status kasbon telah berubah.")
			return
		}
	}
	if kind == "shift_swap" {
		var swap shiftSwap
		swap, err = scanSwap(tx.QueryRowContext(r.Context(), `SELECT `+strings.ReplaceAll(swapColumns, "s.", "")+` FROM shift_swaps WHERE tenant_id=? AND id=? AND request_id=? FOR UPDATE`, c.state.SelectedTenantID, refID.String, requestID))
		if err != nil || swap.Status != "returned" || swap.RequesterID != c.employeeID {
			writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Pertukaran shift tidak tersedia untuk kirim ulang.")
			return
		}
		first, second := swap.InstanceAID, swap.InstanceBID
		if first > second {
			first, second = second, first
		}
		for _, id := range []string{first, second} {
			instance, e := scanInstance(tx.QueryRowContext(r.Context(), `SELECT `+strings.ReplaceAll(instanceColumns, "i.", "")+` FROM shift_instances WHERE tenant_id=? AND id=? FOR UPDATE`, c.state.SelectedTenantID, id))
			if e != nil || instance.Status != "scheduled" || instance.WorkDate < time.Now().In(c.zone).Format("2006-01-02") {
				writeAPIError(w, r, 409, "SHIFT_UNAVAILABLE", "Shift sudah berubah atau berlalu.")
				return
			}
		}
		var occupied int
		if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM attendance_events WHERE tenant_id=? AND shift_instance_id IN (?,?)`, c.state.SelectedTenantID, first, second).Scan(&occupied); err != nil || occupied > 0 {
			writeAPIError(w, r, 409, "SHIFT_IN_USE", "Shift sudah dipakai presensi.")
			return
		}
		result, e := tx.ExecContext(r.Context(), `UPDATE shift_swaps SET status='in_review',version=version+1 WHERE tenant_id=? AND id=? AND request_id=? AND status='returned'`, c.state.SelectedTenantID, refID.String, requestID)
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengirim ulang pertukaran shift.")
			return
		}
		count, _ := result.RowsAffected()
		if count != 1 {
			writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Pertukaran shift sudah berubah.")
			return
		}
	}
	if kind == "attendance_correction" {
		item, e := scanCorrection(tx.QueryRowContext(r.Context(), `SELECT `+correctionColumns+` FROM attendance_corrections c WHERE c.tenant_id=? AND c.id=? AND c.request_id=? AND c.employee_id=? FOR UPDATE`, c.state.SelectedTenantID, refID.String, requestID, c.employeeID))
		if e != nil || item.Status != "returned" {
			writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Koreksi sudah berubah.")
			return
		}
		in := attendanceCorrectionInput{EmployeeID: c.employeeID, ShiftInstanceID: item.ShiftInstanceID, OriginalEventID: item.OriginalEventID, RequestedType: item.RequestedType, RequestedTime: item.RequestedTime.Format(time.RFC3339Nano), Reason: item.Reason, AttachmentFileID: item.AttachmentFileID}
		if value, ok := normalized["reason"].(string); ok {
			in.Reason = value
		}
		if value, ok := normalized["requested_type"].(string); ok {
			in.RequestedType = value
		}
		if value, ok := normalized["requested_time"].(string); ok {
			in.RequestedTime = value
		}
		if value, exists := normalized["original_event_id"]; exists {
			if value == nil {
				in.OriginalEventID = nil
			} else {
				id := value.(string)
				in.OriginalEventID = &id
			}
		}
		if value, exists := normalized["attachment_file_id"]; exists {
			if value == nil {
				in.AttachmentFileID = nil
			} else {
				id := value.(string)
				in.AttachmentFileID = &id
			}
		}
		at, valid := validCorrectionInput(in, c.zone)
		if !valid {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Perbaikan koreksi tidak valid.")
			return
		}
		instance, e := scanInstance(tx.QueryRowContext(r.Context(), `SELECT `+strings.ReplaceAll(instanceColumns, "i.", "")+` FROM shift_instances WHERE tenant_id=? AND id=? AND employee_id=? AND status<>'cancelled' FOR UPDATE`, c.state.SelectedTenantID, in.ShiftInstanceID, c.employeeID))
		if e != nil || at.Before(instance.ExpectedStart.Add(-24*time.Hour)) || at.After(instance.ExpectedEnd.Add(24*time.Hour)) {
			writeAPIError(w, r, 409, "SHIFT_UNAVAILABLE", "Shift koreksi tidak tersedia.")
			return
		}
		if in.OriginalEventID != nil {
			var originalType string
			e = tx.QueryRowContext(r.Context(), `SELECT event_type FROM attendance_events WHERE tenant_id=? AND id=? AND employee_id=? AND shift_instance_id=?`, c.state.SelectedTenantID, *in.OriginalEventID, c.employeeID, in.ShiftInstanceID).Scan(&originalType)
			if e != nil || originalType != in.RequestedType {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Event asli tidak sesuai koreksi.")
				return
			}
		}
		if in.AttachmentFileID != nil {
			var count int
			e = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM files WHERE tenant_id=? AND id=? AND uploaded_by=? AND purpose='selfie' AND scan_status='clean' AND deleted_at IS NULL`, c.state.SelectedTenantID, *in.AttachmentFileID, c.state.User.ID).Scan(&count)
			if e != nil || count != 1 {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Lampiran tidak tersedia.")
				return
			}
		}
		var duplicate int
		e = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM attendance_corrections WHERE tenant_id=? AND shift_instance_id=? AND requested_type=? AND id<>? AND status IN ('in_review','returned')`, c.state.SelectedTenantID, in.ShiftInstanceID, in.RequestedType, item.ID).Scan(&duplicate)
		if e != nil || duplicate > 0 {
			writeAPIError(w, r, 409, "CORRECTION_EXISTS", "Koreksi aktif lain sudah ada.")
			return
		}
		result, e := tx.ExecContext(r.Context(), `UPDATE attendance_corrections SET requested_type=?,requested_time=?,reason=?,original_event_id=?,attachment_file_id=?,status='in_review',version=version+1 WHERE tenant_id=? AND id=? AND request_id=? AND employee_id=? AND status='returned'`, in.RequestedType, at, in.Reason, in.OriginalEventID, in.AttachmentFileID, c.state.SelectedTenantID, refID.String, requestID, c.employeeID)
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengirim ulang koreksi.")
			return
		}
		count, _ := result.RowsAffected()
		if count != 1 {
			writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Koreksi sudah berubah.")
			return
		}
	}
	nextAttempt := attemptNo + 1
	revisionID, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat revisi pengajuan.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO request_revisions (tenant_id,id,request_id,attempt_no,correction_json,submitted_by) VALUES (?,?,?,?,?,?)`, c.state.SelectedTenantID, revisionID, requestID, nextAttempt, string(correctionJSON), c.state.User.ID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan revisi pengajuan.")
		return
	}
	for _, userID := range eligible {
		taskID, err := ids.New()
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat tugas persetujuan.")
			return
		}
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO approval_tasks (tenant_id,id,instance_id,attempt_no,level_no,assignee_user_id,status) VALUES (?,?,?,?,1,?,'pending')`, c.state.SelectedTenantID, taskID, instanceID, nextAttempt, userID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat tugas persetujuan.")
			return
		}
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE approval_instances SET status='in_review',attempt_no=?,current_level=1,version=version+1,started_at=UTC_TIMESTAMP(6),finished_at=NULL WHERE tenant_id=? AND id=?`, nextAttempt, c.state.SelectedTenantID, instanceID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuka ulang proses persetujuan.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE requests SET status='in_review',attempt_no=?,version=version+1,submitted_at=UTC_TIMESTAMP(6),final_at=NULL WHERE tenant_id=? AND id=?`, nextAttempt, c.state.SelectedTenantID, requestID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuka ulang pengajuan.")
		return
	}
	outboxID, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat notifikasi.")
		return
	}
	payload, _ := json.Marshal(map[string]any{"request_id": requestID, "attempt_no": nextAttempt, "approver_user_ids": eligible, "revision_id": revisionID})
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO notification_outbox (tenant_id,id,event_type,aggregate_type,aggregate_id,payload_json,dedupe_key) VALUES (?,?,'approval.resubmitted','request',?,?,?)`, c.state.SelectedTenantID, outboxID, requestID, string(payload), "approval.resubmitted:"+requestID+":"+revisionID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menjadwalkan notifikasi.")
		return
	}
	before := map[string]any{"status": "returned", "attempt_no": attemptNo, "version": version}
	after := map[string]any{"status": "in_review", "attempt_no": nextAttempt, "version": version + 1, "revision_id": revisionID}
	if err := audit(r.Context(), tx, c.state.SelectedTenantID, c.state.User.ID, "resubmit", "requests", requestID, before, after); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengirim ulang pengajuan.")
		return
	}
	writeAPIData(w, 200, map[string]any{"request_id": requestID, "status": "in_review", "attempt_no": nextAttempt, "version": version + 1, "approval_version": instanceVersion + 1, "revision_id": revisionID, "approver_user_ids": eligible}, nil)
}
