package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strings"

	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

type orgTreeNode struct {
	ID       string        `json:"id"`
	Code     string        `json:"code"`
	Name     string        `json:"name"`
	Timezone string        `json:"timezone,omitempty"`
	Children []orgTreeNode `json:"children"`
}

func (a adminAPI) mountOrganizationExtras(r chi.Router, config Options) {
	r.Get("/organization/tree", a.organizationTree)
	r.Get("/locations/tree", a.locationsTree)
	r.Get("/offices/{id}/geofences", a.listOfficeGeofences)
	r.With(checkOrigin(config)).Post("/offices/{id}/geofences", a.createGeofence)
	r.Get("/geofences/{id}", a.getGeofence)
	r.With(checkOrigin(config)).Patch("/geofences/{id}", a.updateGeofence)
	r.With(checkOrigin(config)).Delete("/geofences/{id}", a.deleteGeofence)
	r.With(checkOrigin(config)).Post("/geofences/test-point", a.testGeofencePoint)
}

func (a adminAPI) treeNodes(r *http.Request, tenantID, table, parentColumn string) ([]orgTreeNode, map[string][]orgTreeNode, error) {
	columns := "id,code,name"
	if table == "offices" {
		columns += ",timezone"
	}
	if parentColumn != "" {
		columns += "," + parentColumn
	}
	rows, err := a.db.QueryContext(r.Context(), "SELECT "+columns+" FROM "+table+" WHERE tenant_id=? AND deleted_at IS NULL ORDER BY name,id", tenantID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	items := []orgTreeNode{}
	byParent := map[string][]orgTreeNode{}
	for rows.Next() {
		var item orgTreeNode
		var parent string
		item.Children = []orgTreeNode{}
		dest := []any{&item.ID, &item.Code, &item.Name}
		if table == "offices" {
			dest = append(dest, &item.Timezone)
		}
		if parentColumn != "" {
			dest = append(dest, &parent)
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, nil, err
		}
		items = append(items, item)
		if parentColumn != "" {
			byParent[parent] = append(byParent[parent], item)
		}
	}
	return items, byParent, rows.Err()
}

func (a adminAPI) organizationTree(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "organisasi.lihat")
	if !ok {
		return
	}
	divisions, _, err := a.treeNodes(r, c.tenant.ID, "divisions", "")
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca struktur organisasi.")
		return
	}
	_, departments, err := a.treeNodes(r, c.tenant.ID, "departments", "division_id")
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca struktur organisasi.")
		return
	}
	_, positions, err := a.treeNodes(r, c.tenant.ID, "positions", "department_id")
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca struktur organisasi.")
		return
	}
	for i := range divisions {
		divisions[i].Children = departments[divisions[i].ID]
		if divisions[i].Children == nil {
			divisions[i].Children = []orgTreeNode{}
		}
		for j := range divisions[i].Children {
			divisions[i].Children[j].Children = positions[divisions[i].Children[j].ID]
			if divisions[i].Children[j].Children == nil {
				divisions[i].Children[j].Children = []orgTreeNode{}
			}
		}
	}
	writeAPIData(w, 200, divisions, nil)
}

func (a adminAPI) locationsTree(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "organisasi.lihat")
	if !ok {
		return
	}
	regions, _, err := a.treeNodes(r, c.tenant.ID, "regions", "")
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca lokasi.")
		return
	}
	_, offices, err := a.treeNodes(r, c.tenant.ID, "offices", "region_id")
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca lokasi.")
		return
	}
	for i := range regions {
		regions[i].Children = offices[regions[i].ID]
		if regions[i].Children == nil {
			regions[i].Children = []orgTreeNode{}
		}
	}
	writeAPIData(w, 200, regions, nil)
}

type geoPolygon struct {
	Type        string        `json:"type"`
	Coordinates [][][]float64 `json:"coordinates"`
}
type geofence struct {
	ID       string     `json:"id"`
	OfficeID string     `json:"office_id"`
	Name     string     `json:"name"`
	Polygon  geoPolygon `json:"polygon"`
	IsActive bool       `json:"is_active"`
}
type geoPoint struct{ x, y float64 }

func validPolygon(p geoPolygon) bool {
	if p.Type != "Polygon" || len(p.Coordinates) != 1 {
		return false
	}
	ring := p.Coordinates[0]
	if len(ring) < 4 || len(ring) > 1000 {
		return false
	}
	points := make([]geoPoint, len(ring))
	for i, pair := range ring {
		if len(pair) != 2 || math.IsNaN(pair[0]) || math.IsNaN(pair[1]) || math.IsInf(pair[0], 0) || math.IsInf(pair[1], 0) || pair[0] < -180 || pair[0] > 180 || pair[1] < -90 || pair[1] > 90 {
			return false
		}
		points[i] = geoPoint{pair[0], pair[1]}
	}
	if points[0] != points[len(points)-1] {
		return false
	}
	area := 0.0
	for i := 0; i < len(points)-1; i++ {
		area += points[i].x*points[i+1].y - points[i+1].x*points[i].y
	}
	if math.Abs(area) < 1e-12 {
		return false
	}
	for i := 0; i < len(points)-1; i++ {
		if points[i] == points[i+1] {
			return false
		}
		for j := i + 1; j < len(points)-1; j++ {
			if j == i+1 || (i == 0 && j == len(points)-2) {
				continue
			}
			if segmentsIntersect(points[i], points[i+1], points[j], points[j+1]) {
				return false
			}
		}
	}
	return true
}

func orientation(a, b, c geoPoint) float64 { return (b.x-a.x)*(c.y-a.y) - (b.y-a.y)*(c.x-a.x) }
func onSegment(a, b, c geoPoint) bool {
	return math.Abs(orientation(a, b, c)) < 1e-10 && c.x >= math.Min(a.x, b.x)-1e-10 && c.x <= math.Max(a.x, b.x)+1e-10 && c.y >= math.Min(a.y, b.y)-1e-10 && c.y <= math.Max(a.y, b.y)+1e-10
}
func segmentsIntersect(a, b, c, d geoPoint) bool {
	o1, o2, o3, o4 := orientation(a, b, c), orientation(a, b, d), orientation(c, d, a), orientation(c, d, b)
	if (o1 > 0 && o2 < 0 || o1 < 0 && o2 > 0) && (o3 > 0 && o4 < 0 || o3 < 0 && o4 > 0) {
		return true
	}
	return onSegment(a, b, c) || onSegment(a, b, d) || onSegment(c, d, a) || onSegment(c, d, b)
}
func pointInPolygon(p geoPoint, polygon geoPolygon) bool {
	if !validPolygon(polygon) {
		return false
	}
	ring := polygon.Coordinates[0]
	inside := false
	for i := 0; i < len(ring)-1; i++ {
		a, b := geoPoint{ring[i][0], ring[i][1]}, geoPoint{ring[i+1][0], ring[i+1][1]}
		if onSegment(a, b, p) {
			return true
		}
		if (a.y > p.y) != (b.y > p.y) && p.x < (b.x-a.x)*(p.y-a.y)/(b.y-a.y)+a.x {
			inside = !inside
		}
	}
	return inside
}

func (a adminAPI) readGeofence(r *http.Request, tenantID, id string) (geofence, error) {
	var item geofence
	var polygon []byte
	err := a.db.QueryRowContext(r.Context(), `SELECT id,office_id,name,polygon_json,is_active FROM office_geofences WHERE tenant_id=? AND id=?`, tenantID, id).Scan(&item.ID, &item.OfficeID, &item.Name, &polygon, &item.IsActive)
	if err != nil {
		return item, err
	}
	err = json.Unmarshal(polygon, &item.Polygon)
	return item, err
}
func (a adminAPI) getGeofence(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "organisasi.lihat")
	if !ok {
		return
	}
	item, err := a.readGeofence(r, c.tenant.ID, chi.URLParam(r, "id"))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Geofence tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca geofence.")
		return
	}
	writeAPIData(w, 200, item, nil)
}
func (a adminAPI) listOfficeGeofences(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "organisasi.lihat")
	if !ok {
		return
	}
	officeID := chi.URLParam(r, "id")
	var exists int
	if len(officeID) != 26 || a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM offices WHERE tenant_id=? AND id=?`, c.tenant.ID, officeID).Scan(&exists) != nil || exists != 1 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Kantor tidak ditemukan.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT id,office_id,name,polygon_json,is_active FROM office_geofences WHERE tenant_id=? AND office_id=? ORDER BY name,id`, c.tenant.ID, officeID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca geofence.")
		return
	}
	defer rows.Close()
	items := []geofence{}
	for rows.Next() {
		var item geofence
		var polygon []byte
		if err := rows.Scan(&item.ID, &item.OfficeID, &item.Name, &polygon, &item.IsActive); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca geofence.")
			return
		}
		if err := json.Unmarshal(polygon, &item.Polygon); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Geofence tidak valid.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca geofence.")
		return
	}
	writeAPIData(w, 200, items, nil)
}
func (a adminAPI) createGeofence(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "organisasi.ubah")
	if !ok {
		return
	}
	officeID := chi.URLParam(r, "id")
	var exists int
	if len(officeID) != 26 || a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM offices WHERE tenant_id=? AND id=?`, c.tenant.ID, officeID).Scan(&exists) != nil || exists != 1 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Kantor tidak ditemukan.")
		return
	}
	var body struct {
		Name     string     `json:"name"`
		Polygon  geoPolygon `json:"polygon"`
		IsActive *bool      `json:"is_active"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" || len([]rune(name)) > 160 || !validPolygon(body.Polygon) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Nama atau polygon tidak valid.")
		return
	}
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat geofence.")
		return
	}
	active := true
	if body.IsActive != nil {
		active = *body.IsActive
	}
	polygon, _ := json.Marshal(body.Polygon)
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan geofence.")
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO office_geofences (tenant_id,id,office_id,name,polygon_json,is_active) VALUES (?,?,?,?,?,?)`, c.tenant.ID, id, officeID, name, polygon, active); err != nil {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Geofence tidak dapat disimpan.")
		return
	}
	item := geofence{ID: id, OfficeID: officeID, Name: name, Polygon: body.Polygon, IsActive: active}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "create", "office_geofences", id, nil, item); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan geofence.")
		return
	}
	writeAPIData(w, 201, item, nil)
}
func (a adminAPI) updateGeofence(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "organisasi.ubah")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	old, err := a.readGeofence(r, c.tenant.ID, id)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Geofence tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca geofence.")
		return
	}
	var body struct {
		Name     *string     `json:"name"`
		Polygon  *geoPolygon `json:"polygon"`
		IsActive *bool       `json:"is_active"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	item := old
	if body.Name != nil {
		item.Name = strings.TrimSpace(*body.Name)
	}
	if body.Polygon != nil {
		item.Polygon = *body.Polygon
	}
	if body.IsActive != nil {
		item.IsActive = *body.IsActive
	}
	if item.Name == "" || len([]rune(item.Name)) > 160 || !validPolygon(item.Polygon) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Nama atau polygon tidak valid.")
		return
	}
	polygon, _ := json.Marshal(item.Polygon)
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan geofence.")
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(r.Context(), `UPDATE office_geofences SET name=?,polygon_json=?,is_active=? WHERE tenant_id=? AND id=?`, item.Name, polygon, item.IsActive, c.tenant.ID, id)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan geofence.")
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Geofence tidak ditemukan.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "update", "office_geofences", id, old, item); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan geofence.")
		return
	}
	writeAPIData(w, 200, item, nil)
}
func (a adminAPI) deleteGeofence(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "organisasi.ubah")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	old, err := a.readGeofence(r, c.tenant.ID, id)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Geofence tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca geofence.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menonaktifkan geofence.")
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `UPDATE office_geofences SET is_active=FALSE WHERE tenant_id=? AND id=?`, c.tenant.ID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menonaktifkan geofence.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "delete", "office_geofences", id, old, map[string]any{"is_active": false}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menonaktifkan geofence.")
		return
	}
	w.WriteHeader(204)
}
func (a adminAPI) testGeofencePoint(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "organisasi.lihat")
	if !ok {
		return
	}
	var body struct {
		Lat float64 `json:"lat"`
		Lng float64 `json:"lng"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if math.IsNaN(body.Lat) || math.IsNaN(body.Lng) || body.Lat < -90 || body.Lat > 90 || body.Lng < -180 || body.Lng > 180 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Koordinat tidak valid.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT id,office_id,name,polygon_json FROM office_geofences WHERE tenant_id=? AND is_active=TRUE`, c.tenant.ID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menguji lokasi.")
		return
	}
	defer rows.Close()
	matches := []map[string]string{}
	for rows.Next() {
		var id, officeID, name string
		var raw []byte
		var polygon geoPolygon
		if err := rows.Scan(&id, &officeID, &name, &raw); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menguji lokasi.")
			return
		}
		if json.Unmarshal(raw, &polygon) == nil && pointInPolygon(geoPoint{body.Lng, body.Lat}, polygon) {
			matches = append(matches, map[string]string{"id": id, "office_id": officeID, "name": name})
		}
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menguji lokasi.")
		return
	}
	writeAPIData(w, 200, map[string]any{"inside": len(matches) > 0, "matches": matches}, nil)
}
