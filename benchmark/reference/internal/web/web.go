// Package web holds the HTTP helpers the reference solutions share.
package web

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
)

// ErrNotFound is returned by stores when no item has the ID.
var ErrNotFound = errors.New("not found")

// Page is a list response.
type Page[T any] struct {
	Items  []T `json:"items"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// JSON writes v with the status.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Error writes {"error": message} with the status.
func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, map[string]string{"error": message})
}

// Decode reads a JSON body into dst, responding 400 and returning false if it
// isn't valid JSON.
func Decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err == nil {
		err = json.Unmarshal(body, dst)
	}
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return false
	}
	return true
}

// Paging reads limit and offset: limit defaults to 20 and is capped at 100,
// offset defaults to 0. It responds 400 and returns false for invalid values.
func Paging(w http.ResponseWriter, r *http.Request) (limit, offset int, ok bool) {
	limit, offset = 20, 0
	for name, dst := range map[string]*int{"limit": &limit, "offset": &offset} {
		v := r.URL.Query().Get(name)
		if v == "" {
			continue
		}
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			Error(w, http.StatusBadRequest, name+" must be a non-negative integer")
			return 0, 0, false
		}
		*dst = n
	}
	return min(limit, 100), offset, true
}

// Window returns the part of items the page covers.
func Window[T any](items []T, limit, offset int) []T {
	if offset >= len(items) {
		return []T{}
	}
	return items[offset:min(offset+limit, len(items))]
}

// NewID returns a random ID.
func NewID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Serve registers GET /health and serves mux on PORT.
func Serve(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
