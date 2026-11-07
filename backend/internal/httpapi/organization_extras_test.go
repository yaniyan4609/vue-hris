package httpapi

import "testing"

func TestGeofencePolygonValidationAndBoundary(t *testing.T) {
	valid := geoPolygon{Type: "Polygon", Coordinates: [][][]float64{{{123.9, 48.1}, {124, 48.1}, {124, 48.2}, {123.9, 48.2}, {123.9, 48.1}}}}
	if !validPolygon(valid) {
		t.Fatal("valid polygon rejected")
	}
	for _, item := range []struct {
		point  geoPoint
		inside bool
	}{
		{geoPoint{123.95, 48.15}, true},
		{geoPoint{123.9, 48.15}, true},
		{geoPoint{124.1, 48.15}, false},
	} {
		if got := pointInPolygon(item.point, valid); got != item.inside {
			t.Fatalf("point %#v: got %v", item.point, got)
		}
	}
	bowtie := geoPolygon{Type: "Polygon", Coordinates: [][][]float64{{{0, 0}, {1, 1}, {0, 1}, {1, 0}, {0, 0}}}}
	if validPolygon(bowtie) {
		t.Fatal("self intersecting polygon accepted")
	}
	open := geoPolygon{Type: "Polygon", Coordinates: [][][]float64{{{0, 0}, {1, 0}, {1, 1}, {0, 1}}}}
	if validPolygon(open) {
		t.Fatal("open polygon accepted")
	}
}
