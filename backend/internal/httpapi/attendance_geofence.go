package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
)

// attendanceGeofence limits candidate polygons to the roster office, current
// base office, or an explicit active allowed-office assignment on work_date.
func (a adminAPI) attendanceGeofence(r *http.Request, tx *sql.Tx, tenantID, employeeID, workDate, rosterOfficeID string, lat, lng float64) (locationMatch, string, error) {
	rows, err := tx.QueryContext(r.Context(), `SELECT g.id,o.id,o.name,g.polygon_json FROM office_geofences g JOIN offices o ON o.tenant_id=g.tenant_id AND o.id=g.office_id WHERE g.tenant_id=? AND g.is_active=TRUE AND (o.id=? OR o.id=(SELECT a.office_id FROM employee_assignments a WHERE a.tenant_id=? AND a.employee_id=? AND a.current_flag=TRUE) OR EXISTS (SELECT 1 FROM employee_allowed_offices ao WHERE ao.tenant_id=g.tenant_id AND ao.office_id=g.office_id AND ao.employee_id=? AND ao.revoked_at IS NULL AND ao.valid_from<=? AND (ao.valid_to IS NULL OR ao.valid_to>=?))) ORDER BY o.id,g.id`, tenantID, rosterOfficeID, tenantID, employeeID, employeeID, workDate, workDate)
	if err != nil {
		return locationMatch{}, "", err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var match locationMatch
		var raw []byte
		var polygon geoPolygon
		if err = rows.Scan(&match.GeofenceID, &match.OfficeID, &match.OfficeName, &raw); err != nil {
			return locationMatch{}, "", err
		}
		if err = json.Unmarshal(raw, &polygon); err != nil || !validPolygon(polygon) {
			return locationMatch{}, "", errors.New("invalid geofence")
		}
		count++
		if pointInPolygon(geoPoint{lng, lat}, polygon) {
			return match, "inside", nil
		}
	}
	if err = rows.Err(); err != nil {
		return locationMatch{}, "", err
	}
	if count == 0 {
		return locationMatch{}, "unknown", nil
	}
	return locationMatch{}, "outside", nil
}
