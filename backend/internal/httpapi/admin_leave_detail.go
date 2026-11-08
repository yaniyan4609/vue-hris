package httpapi

import (
	"database/sql"
	"net/http"
	"time"
)

func (a adminAPI) attachAdminLeaveApproval(r *http.Request, tenantID string, item map[string]any) error {
	requestID, ok := item["request_id"].(string)
	if !ok || requestID == "" {
		return nil
	}
	request, err := scanRequest(a.db.QueryRowContext(r.Context(), `SELECT `+requestColumns+requestJoin+` WHERE q.tenant_id=? AND q.id=? AND q.request_type='leave' AND q.ref_table='leave_requests' AND q.ref_id=?`, tenantID, requestID, item["id"]))
	if err != nil {
		return err
	}
	if err = a.attachRequestRevisions(r, tenantID, &request); err != nil {
		return err
	}
	tasks := []map[string]any{}
	actions := []map[string]any{}
	if request.Approval != nil {
		rows, err := a.db.QueryContext(r.Context(), `SELECT id,attempt_no,level_no,assignee_user_id,status,decided_at FROM approval_tasks WHERE tenant_id=? AND instance_id=? ORDER BY attempt_no,level_no,created_at,id`, tenantID, request.Approval.ID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id, user, status string
			var attempt uint64
			var level int
			var decided sql.NullTime
			if err = rows.Scan(&id, &attempt, &level, &user, &status, &decided); err != nil {
				break
			}
			var decidedAt any
			if decided.Valid {
				decidedAt = decided.Time
			}
			tasks = append(tasks, map[string]any{"id": id, "attempt_no": attempt, "level_no": level, "assignee_user_id": user, "status": status, "decided_at": decidedAt})
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return err
		}
		rows, err = a.db.QueryContext(r.Context(), `SELECT id,level_no,actor_user_id,action,reason,created_at FROM approval_actions WHERE tenant_id=? AND instance_id=? ORDER BY created_at,id`, tenantID, request.Approval.ID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id, actor, action string
			var level int
			var reason sql.NullString
			var created time.Time
			if err = rows.Scan(&id, &level, &actor, &action, &reason, &created); err != nil {
				break
			}
			var reasonValue any
			if reason.Valid {
				reasonValue = reason.String
			}
			actions = append(actions, map[string]any{"id": id, "level_no": level, "actor_user_id": actor, "action": action, "reason": reasonValue, "created_at": created})
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return err
		}
	}
	item["request"] = request
	item["tasks"] = tasks
	item["actions"] = actions
	return nil
}
