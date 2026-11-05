package httpapi

import (
	"context"
	"database/sql"
	"time"
)

func platformAccess(ctx context.Context, db *sql.DB, userID string) (map[string]accessEntry, bool, error) {
	result := map[string]accessEntry{}
	rows, err := db.QueryContext(ctx, `SELECT code FROM platform_permission_registry ORDER BY code`)
	if err != nil {
		return nil, false, err
	}
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			rows.Close()
			return nil, false, err
		}
		result[code] = accessEntry{Reason: "permission_missing"}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, false, err
	}
	rows.Close()
	date := time.Now().In(time.FixedZone("WIB", 7*3600)).Format("2006-01-02")
	rows, err = db.QueryContext(ctx, `SELECT pr.code,p.permission_code FROM platform_user_roles pur JOIN platform_roles pr ON pr.id=pur.role_id LEFT JOIN platform_role_permissions p ON p.role_id=pr.id WHERE pur.user_id=? AND pur.valid_from<=? AND (pur.valid_until IS NULL OR pur.valid_until>=?)`, userID, date, date)
	if err != nil {
		return nil, false, err
	}
	superadmin := false
	for rows.Next() {
		var role string
		var code sql.NullString
		if err := rows.Scan(&role, &code); err != nil {
			rows.Close()
			return nil, false, err
		}
		if role == "SUPERADMIN" {
			superadmin = true
		}
		if code.Valid {
			item := result[code.String]
			item.Allowed, item.Reason = true, "allowed"
			item.Sources = append(item.Sources, "role:"+role)
			result[code.String] = item
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, false, err
	}
	rows.Close()
	rows, err = db.QueryContext(ctx, `SELECT permission_code,decision FROM platform_user_permission_overrides WHERE user_id=?`, userID)
	if err != nil {
		return nil, false, err
	}
	for rows.Next() {
		var code, decision string
		if err := rows.Scan(&code, &decision); err != nil {
			rows.Close()
			return nil, false, err
		}
		item, known := result[code]
		if !known {
			continue
		}
		if decision == "deny" {
			item = accessEntry{Reason: "user_deny", Override: "deny"}
		} else if decision == "allow" {
			item.Allowed, item.Reason = true, "allowed"
			item.Sources = append(item.Sources, "user:allow")
			item.Override = "allow"
		}
		result[code] = item
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, false, err
	}
	rows.Close()
	return result, superadmin, nil
}
