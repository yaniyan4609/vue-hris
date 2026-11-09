package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/ids"
)

type overtimeDecision struct {
	Kind            string
	Action          string
	Reason          string
	Data            json.RawMessage
	RequestID       string
	RefID           string
	InstanceID      string
	FlowID          string
	TaskID          string
	Requester       string
	Employee        employee
	Target          access.Target
	Level           int
	AttemptNo       uint64
	RequestVersion  uint64
	InstanceVersion uint64
}

func (a adminAPI) decideOvertimeApproval(w http.ResponseWriter, r *http.Request, tx *sql.Tx, c adminContext, d overtimeDecision) {
	var proposed int
	var domainStatus string
	if d.Kind == "leave" {
		if err := tx.QueryRowContext(r.Context(), `SELECT status FROM leave_requests WHERE tenant_id=? AND id=? AND request_id=? AND employee_id=? FOR UPDATE`, c.tenant.ID, d.RefID, d.RequestID, d.Employee.ID).Scan(&domainStatus); err != nil || domainStatus != "in_review" {
			writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Status data cuti tidak sesuai pengajuan.")
			return
		}
	} else if d.Kind == "reimbursement" {
		if err := tx.QueryRowContext(r.Context(), `SELECT status FROM reimbursements WHERE tenant_id=? AND id=? AND request_id=? AND employee_id=? FOR UPDATE`, c.tenant.ID, d.RefID, d.RequestID, d.Employee.ID).Scan(&domainStatus); err != nil || domainStatus != "in_review" {
			writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Status klaim tidak sesuai pengajuan.")
			return
		}
	} else if d.Kind == "loan" {
		if err := tx.QueryRowContext(r.Context(), `SELECT status FROM employee_loans WHERE tenant_id=? AND id=? AND request_id=? AND employee_id=? FOR UPDATE`, c.tenant.ID, d.RefID, d.RequestID, d.Employee.ID).Scan(&domainStatus); err != nil || domainStatus != "in_review" {
			writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Status kasbon tidak sesuai pengajuan.")
			return
		}
	} else if d.Kind == "mutation" || d.Kind == "promotion" {
		if err := tx.QueryRowContext(r.Context(), `SELECT status FROM employee_changes WHERE tenant_id=? AND id=? AND request_id=? AND employee_id=? FOR UPDATE`, c.tenant.ID, d.RefID, d.RequestID, d.Employee.ID).Scan(&domainStatus); err != nil || domainStatus != "in_review" {
			writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Status perubahan karyawan tidak sesuai pengajuan.")
			return
		}
	} else {
		if err := tx.QueryRowContext(r.Context(), `SELECT proposed_minutes,status FROM overtime_requests WHERE tenant_id=? AND id=? AND request_id=? AND employee_id=? FOR UPDATE`, c.tenant.ID, d.RefID, d.RequestID, d.Employee.ID).Scan(&proposed, &domainStatus); err != nil || domainStatus != "in_review" {
			writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Status data lembur tidak sesuai pengajuan.")
			return
		}
	}
	var maxLevel int
	if err := tx.QueryRowContext(r.Context(), `SELECT COALESCE(MAX(level_no),0) FROM approval_flow_steps WHERE tenant_id=? AND flow_id=?`, c.tenant.ID, d.FlowID).Scan(&maxLevel); err != nil || maxLevel < d.Level {
		writeAPIError(w, r, 409, "FLOW_INVALID", "Tahap alur persetujuan tidak valid.")
		return
	}
	final := d.Action == "reject" || d.Level == maxLevel
	var approvedMinutes *int
	var actionData any
	if d.Kind == "overtime" && d.Action == "approve" && final {
		var fields map[string]json.RawMessage
		var data struct {
			ApprovedMinutes *int `json:"approved_minutes"`
		}
		if len(d.Data) == 0 || json.Unmarshal(d.Data, &fields) != nil || len(fields) != 1 || json.Unmarshal(d.Data, &data) != nil || data.ApprovedMinutes == nil || *data.ApprovedMinutes < 1 || *data.ApprovedMinutes > proposed {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Menit lembur disetujui harus 1 sampai menit yang diusulkan.")
			return
		}
		approvedMinutes = data.ApprovedMinutes
		normalized, _ := json.Marshal(map[string]int{"approved_minutes": *approvedMinutes})
		actionData = string(normalized)
	} else if len(d.Data) > 0 && string(d.Data) != "null" {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Data keputusan tidak sesuai tahap.")
		return
	}
	nextAssignees := []string{}
	if d.Action == "approve" && !final {
		step, err := scanFlowStep(tx.QueryRowContext(r.Context(), `SELECT id,level_no,approver_kind,approver_ref_id,required_permission,sla_hours FROM approval_flow_steps WHERE tenant_id=? AND flow_id=? AND level_no=?`, c.tenant.ID, d.FlowID, d.Level+1))
		if err != nil || (step.RequiredPermission != nil && *step.RequiredPermission != approvalPermissions[d.Kind]) {
			writeAPIError(w, r, 409, "FLOW_INVALID", "Tahap berikutnya tidak valid.")
			return
		}
		zone, err := time.LoadLocation(c.tenant.Timezone)
		if err != nil {
			zone = time.UTC
		}
		today := time.Now().In(zone).Format("2006-01-02")
		candidates, err := a.stepApproverCandidates(r, c.tenant.ID, today, d.Employee, step)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menentukan approver tahap berikutnya.")
			return
		}
		nextAssignees, err = a.eligibleApprovers(r, c.tenant.ID, c.tenant.Timezone, d.Employee, d.Target, approvalPermissions[d.Kind], candidates, d.Kind)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa hak approver berikutnya.")
			return
		}
		nextAssignees = removeApprover(nextAssignees, d.Requester)
		if len(nextAssignees) == 0 {
			var fallback sql.NullString
			if err := tx.QueryRowContext(r.Context(), `SELECT fallback_role_id FROM approval_flows WHERE tenant_id=? AND id=?`, c.tenant.ID, d.FlowID).Scan(&fallback); err != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa fallback approver.")
				return
			}
			if fallback.Valid {
				candidates, err = a.roleApproverCandidates(r, c.tenant.ID, fallback.String, today)
				if err != nil {
					writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menentukan fallback approver.")
					return
				}
				nextAssignees, err = a.eligibleApprovers(r, c.tenant.ID, c.tenant.Timezone, d.Employee, d.Target, approvalPermissions[d.Kind], candidates, d.Kind)
				if err != nil {
					writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa hak fallback.")
					return
				}
				nextAssignees = removeApprover(nextAssignees, d.Requester)
			}
		}
		if len(nextAssignees) == 0 {
			writeAPIError(w, r, 409, "NO_APPROVER", "Belum ada approver yang berhak untuk tahap berikutnya.")
			return
		}
	}
	taskStatus := "approved"
	siblingStatus := "skipped"
	if d.Action == "reject" {
		taskStatus, siblingStatus = "rejected", "cancelled"
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE approval_tasks SET status=?,decided_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND id=? AND status='pending'`, taskStatus, c.tenant.ID, d.TaskID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menutup tugas persetujuan.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE approval_tasks SET status=?,decided_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND instance_id=? AND attempt_no=? AND level_no=? AND status='pending'`, siblingStatus, c.tenant.ID, d.InstanceID, d.AttemptNo, d.Level); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menutup tugas lain.")
		return
	}
	actionID, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat riwayat keputusan.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO approval_actions (tenant_id,id,instance_id,task_id,level_no,actor_user_id,action,reason,data_json) VALUES (?,?,?,?,?,?,?,?,?)`, c.tenant.ID, actionID, d.InstanceID, d.TaskID, d.Level, c.state.User.ID, d.Action, d.Reason, actionData); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat keputusan.")
		return
	}
	resultStatus := "in_review"
	nextLevel := d.Level
	if d.Action == "reject" {
		resultStatus = "rejected"
	} else if final {
		resultStatus = "approved"
	} else {
		nextLevel++
	}
	if d.Kind == "leave" && resultStatus == "approved" {
		if err := approveLeaveLedger(r, tx, c.tenant.ID, d); err != nil {
			writeLeaveIssue(w, r, err)
			return
		}
		if _, err := tx.ExecContext(r.Context(), `UPDATE leave_requests SET status='approved' WHERE tenant_id=? AND id=? AND request_id=? AND status='in_review'`, c.tenant.ID, d.RefID, d.RequestID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyetujui cuti.")
			return
		}
	} else if d.Kind == "leave" && resultStatus == "rejected" {
		if _, err := tx.ExecContext(r.Context(), `UPDATE leave_requests SET status='rejected' WHERE tenant_id=? AND id=? AND request_id=? AND status='in_review'`, c.tenant.ID, d.RefID, d.RequestID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menolak cuti.")
			return
		}
	} else if d.Kind == "overtime" && resultStatus == "approved" {
		if _, err := tx.ExecContext(r.Context(), `UPDATE overtime_requests SET status='approved',approved_minutes=? WHERE tenant_id=? AND id=? AND request_id=? AND status='in_review'`, approvedMinutes, c.tenant.ID, d.RefID, d.RequestID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyetujui lembur.")
			return
		}
	} else if d.Kind == "overtime" && resultStatus == "rejected" {
		if _, err := tx.ExecContext(r.Context(), `UPDATE overtime_requests SET status='rejected' WHERE tenant_id=? AND id=? AND request_id=? AND status='in_review'`, c.tenant.ID, d.RefID, d.RequestID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menolak lembur.")
			return
		}
	} else if d.Kind == "reimbursement" && (resultStatus == "approved" || resultStatus == "rejected") {
		result, err := tx.ExecContext(r.Context(), `UPDATE reimbursements SET status=? WHERE tenant_id=? AND id=? AND request_id=? AND status='in_review'`, resultStatus, c.tenant.ID, d.RefID, d.RequestID)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memperbarui keputusan klaim.")
			return
		}
		count, _ := result.RowsAffected()
		if count != 1 {
			writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Status klaim telah berubah.")
			return
		}
	} else if d.Kind == "loan" && (resultStatus == "approved" || resultStatus == "rejected") {
		result, err := tx.ExecContext(r.Context(), `UPDATE employee_loans SET status=? WHERE tenant_id=? AND id=? AND request_id=? AND status='in_review'`, resultStatus, c.tenant.ID, d.RefID, d.RequestID)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memperbarui keputusan kasbon.")
			return
		}
		count, _ := result.RowsAffected()
		if count != 1 {
			writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Status kasbon telah berubah.")
			return
		}
	} else if (d.Kind == "mutation" || d.Kind == "promotion") && (resultStatus == "approved" || resultStatus == "rejected") {
		if resultStatus == "approved" {
			var sourceID, currentID string
			var effectiveDate time.Time
			if err := tx.QueryRowContext(r.Context(), `SELECT source_assignment_id,effective_date FROM employee_changes WHERE tenant_id=? AND id=?`, c.tenant.ID, d.RefID).Scan(&sourceID, &effectiveDate); err != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa sumber perubahan.")
				return
			}
			if err := tx.QueryRowContext(r.Context(), `SELECT id FROM employee_assignments WHERE tenant_id=? AND employee_id=? AND current_flag=1 FOR UPDATE`, c.tenant.ID, d.Employee.ID).Scan(&currentID); err != nil || currentID != sourceID {
				writeAPIError(w, r, 409, "ASSIGNMENT_CHANGED", "Penugasan karyawan sudah berubah.")
				return
			}
			if effectiveDate.Format("2006-01-02") < tenantToday(c.tenant.Timezone) {
				writeAPIError(w, r, 409, "DATE_CONFLICT", "Tanggal efektif perubahan sudah berlalu.")
				return
			}
		}
		result, err := tx.ExecContext(r.Context(), `UPDATE employee_changes SET status=?,version=version+1 WHERE tenant_id=? AND id=? AND request_id=? AND status='in_review'`, resultStatus, c.tenant.ID, d.RefID, d.RequestID)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat keputusan perubahan karyawan.")
			return
		}
		count, _ := result.RowsAffected()
		if count != 1 {
			writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Status perubahan karyawan telah berubah.")
			return
		}
	}
	for _, userID := range nextAssignees {
		nextTaskID, err := ids.New()
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat tugas berikutnya.")
			return
		}
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO approval_tasks (tenant_id,id,instance_id,attempt_no,level_no,assignee_user_id,status) VALUES (?,?,?,?,?,?,'pending')`, c.tenant.ID, nextTaskID, d.InstanceID, d.AttemptNo, nextLevel, userID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat tugas berikutnya.")
			return
		}
	}
	if resultStatus == "in_review" {
		_, err = tx.ExecContext(r.Context(), `UPDATE approval_instances SET current_level=?,version=version+1 WHERE tenant_id=? AND id=?`, nextLevel, c.tenant.ID, d.InstanceID)
	} else {
		_, err = tx.ExecContext(r.Context(), `UPDATE approval_instances SET status=?,version=version+1,finished_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND id=?`, resultStatus, c.tenant.ID, d.InstanceID)
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memperbarui proses persetujuan.")
		return
	}
	if resultStatus == "in_review" {
		_, err = tx.ExecContext(r.Context(), `UPDATE requests SET version=version+1 WHERE tenant_id=? AND id=?`, c.tenant.ID, d.RequestID)
	} else {
		_, err = tx.ExecContext(r.Context(), `UPDATE requests SET status=?,version=version+1,final_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND id=?`, resultStatus, c.tenant.ID, d.RequestID)
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memperbarui pengajuan.")
		return
	}
	outboxID, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat notifikasi.")
		return
	}
	payload, _ := json.Marshal(map[string]any{"request_id": d.RequestID, "requester_user_id": d.Requester, "attempt_no": d.AttemptNo, "action_id": actionID, "next_approver_user_ids": nextAssignees})
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO notification_outbox (tenant_id,id,event_type,aggregate_type,aggregate_id,payload_json,dedupe_key) VALUES (?,?,?,'request',?,?,?)`, c.tenant.ID, outboxID, "approval."+resultStatus, d.RequestID, string(payload), "approval."+resultStatus+":"+d.RequestID+":"+actionID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menjadwalkan notifikasi.")
		return
	}
	before := map[string]any{"status": "in_review", "level_no": d.Level, "attempt_no": d.AttemptNo, "version": d.RequestVersion}
	after := map[string]any{"status": resultStatus, "level_no": nextLevel, "attempt_no": d.AttemptNo, "version": d.RequestVersion + 1, "action": d.Action, "approved_minutes": approvedMinutes}
	if err := audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, d.Action, "requests", d.RequestID, before, after); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	result := map[string]any{"request_id": d.RequestID, "status": resultStatus, "attempt_no": d.AttemptNo, "version": d.RequestVersion + 1, "approval_version": d.InstanceVersion + 1, "current_level": nextLevel, "action_id": actionID}
	if err := persistApprovalBulkSuccess(r.Context(), tx, result); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan hasil batch persetujuan.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan keputusan.")
		return
	}
	writeAPIData(w, 200, result, nil)
}
