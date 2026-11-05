package httpapi

import (
	"net/http"
	"strconv"
)

type pageOptions struct {
	Page     int
	PageSize int
	Offset   int
}

type pageResult[T any] struct {
	Items    []T `json:"items"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

func readPage(w http.ResponseWriter, r *http.Request) (pageOptions, bool) {
	page, size := 1, 15
	var err error
	if value := r.URL.Query().Get("page"); value != "" {
		page, err = strconv.Atoi(value)
		if err != nil || page < 1 || page > 1000000 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Halaman tidak valid"})
			return pageOptions{}, false
		}
	}
	if value := r.URL.Query().Get("pageSize"); value != "" {
		size, err = strconv.Atoi(value)
		if err != nil || size < 1 || size > 100 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Jumlah per halaman harus 1–100"})
			return pageOptions{}, false
		}
	}
	return pageOptions{Page: page, PageSize: size, Offset: (page - 1) * size}, true
}

func searchTerm(w http.ResponseWriter, r *http.Request) (string, bool) {
	value := r.URL.Query().Get("q")
	if len([]rune(value)) > 100 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Pencarian terlalu panjang"})
		return "", false
	}
	return value, true
}
