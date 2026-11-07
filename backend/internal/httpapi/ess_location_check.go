package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"time"
)

type locationMatch struct {
	GeofenceID string `json:"geofence_id"`
	OfficeID   string `json:"office_id"`
	OfficeName string `json:"office_name"`
}

const maxAttendanceAccuracyM = 100.0

func (a adminAPI) essLocationCheck(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	var body struct {
		Lat       *float64 `json:"lat"`
		Lng       *float64 `json:"lng"`
		AccuracyM *float64 `json:"accuracy_m"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if body.Lat == nil || body.Lng == nil || body.AccuracyM == nil || math.IsNaN(*body.Lat) || math.IsNaN(*body.Lng) || math.IsNaN(*body.AccuracyM) || math.IsInf(*body.Lat, 0) || math.IsInf(*body.Lng, 0) || math.IsInf(*body.AccuracyM, 0) || *body.Lat < -90 || *body.Lat > 90 || *body.Lng < -180 || *body.Lng > 180 || *body.AccuracyM <= 0 || *body.AccuracyM > 999999.99 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Koordinat atau akurasi tidak valid.")
		return
	}
	if *body.AccuracyM > maxAttendanceAccuracyM {
		writeAPIData(w, 200, map[string]any{"status": "unknown", "matches": []locationMatch{}, "reason": "GPS_ACCURACY_TOO_LOW", "accuracy_m": *body.AccuracyM, "max_accuracy_m": maxAttendanceAccuracyM}, nil)
		return
	}
	now := time.Now().UTC()
	today := now.In(c.zone).Format("2006-01-02")
	yesterday := now.In(c.zone).AddDate(0, 0, -1).Format("2006-01-02")
	var snapshot []byte
	err := a.db.QueryRowContext(r.Context(), `SELECT i.rule_snapshot FROM shift_instances i WHERE i.tenant_id=? AND i.employee_id=? AND i.status<>'cancelled' AND (i.work_date=? OR (i.work_date=? AND ? BETWEEN i.clock_in_window_start_at AND i.clock_out_window_end_at)) ORDER BY CASE WHEN ? BETWEEN i.clock_in_window_start_at AND i.clock_out_window_end_at THEN 0 ELSE 1 END,i.work_date DESC LIMIT 1`, c.state.SelectedTenantID, c.employeeID, today, yesterday, now, now).Scan(&snapshot)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIData(w, 200, map[string]any{"status": "unknown", "matches": []locationMatch{}, "reason": "NO_SHIFT"}, nil)
			return
		}
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca shift.")
		return
	}
	var rule struct {
		Shift struct {
			FlexLocation    bool `json:"flex_location"`
			RequireLocation bool `json:"require_location"`
		} `json:"shift"`
	}
	if json.Unmarshal(snapshot, &rule) != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Snapshot shift tidak valid.")
		return
	}
	if rule.Shift.FlexLocation || !rule.Shift.RequireLocation {
		writeAPIData(w, 200, map[string]any{"status": "flexible", "matches": []locationMatch{}}, nil)
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT g.id,o.id,o.name,g.polygon_json FROM office_geofences g JOIN offices o ON o.tenant_id=g.tenant_id AND o.id=g.office_id WHERE g.tenant_id=? AND g.is_active=TRUE AND (o.id=(SELECT a.office_id FROM employee_assignments a WHERE a.tenant_id=? AND a.employee_id=? AND a.current_flag=TRUE) OR EXISTS (SELECT 1 FROM employee_allowed_offices ao WHERE ao.tenant_id=g.tenant_id AND ao.office_id=g.office_id AND ao.employee_id=? AND ao.revoked_at IS NULL AND ao.valid_from<=? AND (ao.valid_to IS NULL OR ao.valid_to>=?))) ORDER BY o.id,g.id`, c.state.SelectedTenantID, c.state.SelectedTenantID, c.employeeID, c.employeeID, today, today)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca geofence.")
		return
	}
	defer rows.Close()
	matches := []locationMatch{}
	count := 0
	for rows.Next() {
		var match locationMatch
		var raw []byte
		var polygon geoPolygon
		if err := rows.Scan(&match.GeofenceID, &match.OfficeID, &match.OfficeName, &raw); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca geofence.")
			return
		}
		if err := json.Unmarshal(raw, &polygon); err != nil || !validPolygon(polygon) {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Geofence tidak valid.")
			return
		}
		count++
		if pointInPolygon(geoPoint{*body.Lng, *body.Lat}, polygon) {
			matches = append(matches, match)
		}
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca geofence.")
		return
	}
	status := "outside"
	if count == 0 {
		status = "unknown"
	} else if len(matches) > 0 {
		status = "inside"
	}
	writeAPIData(w, 200, map[string]any{"status": status, "matches": matches, "accuracy_m": *body.AccuracyM}, nil)
}
