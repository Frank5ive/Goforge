// Command store is the reference solution for the store task. Every resource
// is handled by the same code, described by its fields, and stored as JSON in
// its own PostgreSQL table.
package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/Frank5ive/goforge/benchmark/reference/internal/web"
)

type kind int

const (
	text kind = iota
	number
	integer
	boolean
)

type field struct {
	name     string
	kind     kind
	required bool
	email    bool // a text field that must be an email address
}

type resource struct {
	table  string
	fields []field
}

var resources = []resource{
	{"categories", []field{{name: "name", required: true}, {name: "description"}}},
	{"products", []field{{name: "name", required: true}, {name: "sku", required: true},
		{name: "price", kind: number}, {name: "in_stock", kind: boolean}, {name: "category_id", required: true}}},
	{"customers", []field{{name: "name", required: true}, {name: "email", required: true, email: true}}},
	{"suppliers", []field{{name: "name", required: true}, {name: "country", required: true}, {name: "phone"}}},
	{"warehouses", []field{{name: "name", required: true}, {name: "city", required: true}, {name: "capacity", kind: integer}}},
}

var errInvalid = errors.New("invalid")

// validate fills in defaults and checks the rules, returning an error that
// wraps errInvalid.
func (r resource) validate(data map[string]any) error {
	for _, f := range r.fields {
		v, ok := data[f.name]
		if !ok || v == nil {
			switch f.kind {
			case text:
				v = ""
			case number, integer:
				v = 0.0
			case boolean:
				v = false
			}
			data[f.name] = v
		}
		switch f.kind {
		case text:
			s, ok := v.(string)
			if !ok {
				return fmt.Errorf("%w: %s must be a string", errInvalid, f.name)
			}
			if f.required && strings.TrimSpace(s) == "" {
				return fmt.Errorf("%w: %s is required", errInvalid, f.name)
			}
			if f.email {
				user, domain, found := strings.Cut(s, "@")
				if !found || user == "" || domain == "" || strings.Contains(domain, "@") {
					return fmt.Errorf("%w: %s is not an email address", errInvalid, f.name)
				}
			}
		case number, integer:
			n, ok := v.(float64)
			if !ok || n < 0 || (f.kind == integer && n != math.Trunc(n)) {
				return fmt.Errorf("%w: %s must be a number, 0 or more", errInvalid, f.name)
			}
		case boolean:
			if _, ok := v.(bool); !ok {
				return fmt.Errorf("%w: %s must be true or false", errInvalid, f.name)
			}
		}
	}
	// Keep only the resource's own fields
	for key := range data {
		if !r.has(key) {
			delete(data, key)
		}
	}
	return nil
}

func (r resource) has(name string) bool {
	for _, f := range r.fields {
		if f.name == name {
			return true
		}
	}
	return false
}

type server struct{ db *sql.DB }

func (s *server) migrate() error {
	for _, r := range resources {
		if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS ` + r.table + ` (
			seq        BIGSERIAL PRIMARY KEY,
			id         TEXT NOT NULL UNIQUE,
			data       JSONB NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`); err != nil {
			return err
		}
	}
	return nil
}

// item is a stored row as the API returns it.
func item(id string, data []byte, created, updated time.Time) (map[string]any, error) {
	m := map[string]any{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	m["id"], m["created_at"], m["updated_at"] = id, created.UTC(), updated.UTC()
	return m, nil
}

func (s *server) get(r resource, id string) (map[string]any, error) {
	var data []byte
	var created, updated time.Time
	err := s.db.QueryRow(`SELECT data, created_at, updated_at FROM `+r.table+` WHERE id = $1`, id).Scan(&data, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, web.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return item(id, data, created, updated)
}

// check enforces the rules that involve other resources.
func (s *server) check(r resource, data map[string]any) error {
	if r.table == "products" {
		cat, _ := data["category_id"].(string)
		if _, err := s.get(resources[0], cat); errors.Is(err, web.ErrNotFound) {
			return fmt.Errorf("%w: category_id is not an existing category", errInvalid)
		} else if err != nil {
			return err
		}
	}
	return nil
}

func (s *server) respond(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, errInvalid):
		web.Error(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, web.ErrNotFound):
		web.Error(w, http.StatusNotFound, "not found")
	default:
		web.Error(w, http.StatusInternalServerError, err.Error())
	}
	return true
}

func (s *server) register(mux *http.ServeMux, r resource) {
	path := "/" + r.table
	mux.HandleFunc("POST "+path, func(w http.ResponseWriter, req *http.Request) {
		data := map[string]any{}
		if !web.Decode(w, req, &data) {
			return
		}
		if err := r.validate(data); s.respond(w, err) {
			return
		}
		if s.respond(w, s.check(r, data)) {
			return
		}
		id, now := web.NewID(), time.Now().UTC()
		raw, _ := json.Marshal(data)
		if _, err := s.db.Exec(`INSERT INTO `+r.table+` (id, data, created_at, updated_at) VALUES ($1, $2, $3, $3)`, id, raw, now); s.respond(w, err) {
			return
		}
		m, err := item(id, raw, now, now)
		if s.respond(w, err) {
			return
		}
		web.JSON(w, http.StatusCreated, m)
	})
	mux.HandleFunc("GET "+path, func(w http.ResponseWriter, req *http.Request) {
		limit, offset, ok := web.Paging(w, req)
		if !ok {
			return
		}
		var total int
		if s.respond(w, s.db.QueryRow(`SELECT COUNT(*) FROM `+r.table).Scan(&total)) {
			return
		}
		rows, err := s.db.Query(`SELECT id, data, created_at, updated_at FROM `+r.table+` ORDER BY seq LIMIT $1 OFFSET $2`, limit, offset)
		if s.respond(w, err) {
			return
		}
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var id string
			var data []byte
			var created, updated time.Time
			if s.respond(w, rows.Scan(&id, &data, &created, &updated)) {
				return
			}
			m, err := item(id, data, created, updated)
			if s.respond(w, err) {
				return
			}
			items = append(items, m)
		}
		web.JSON(w, http.StatusOK, web.Page[map[string]any]{Items: items, Total: total, Limit: limit, Offset: offset})
	})
	mux.HandleFunc("GET "+path+"/{id}", func(w http.ResponseWriter, req *http.Request) {
		m, err := s.get(r, req.PathValue("id"))
		if s.respond(w, err) {
			return
		}
		web.JSON(w, http.StatusOK, m)
	})
	update := func(full bool) http.HandlerFunc {
		return func(w http.ResponseWriter, req *http.Request) {
			in := map[string]any{}
			if !web.Decode(w, req, &in) {
				return
			}
			current, err := s.get(r, req.PathValue("id"))
			if s.respond(w, err) {
				return
			}
			created := current["created_at"].(time.Time)
			data := in
			if !full {
				// Start from a copy of the stored fields, then apply the changes
				data = map[string]any{}
				for k, v := range current {
					data[k] = v
				}
				for k, v := range in {
					data[k] = v
				}
			}
			if err := r.validate(data); s.respond(w, err) {
				return
			}
			if s.respond(w, s.check(r, data)) {
				return
			}
			now := time.Now().UTC()
			raw, _ := json.Marshal(data)
			if _, err := s.db.Exec(`UPDATE `+r.table+` SET data = $1, updated_at = $2 WHERE id = $3`, raw, now, req.PathValue("id")); s.respond(w, err) {
				return
			}
			m, err := item(req.PathValue("id"), raw, created, now)
			if s.respond(w, err) {
				return
			}
			web.JSON(w, http.StatusOK, m)
		}
	}
	mux.HandleFunc("PUT "+path+"/{id}", update(true))
	mux.HandleFunc("PATCH "+path+"/{id}", update(false))
	mux.HandleFunc("DELETE "+path+"/{id}", func(w http.ResponseWriter, req *http.Request) {
		id := req.PathValue("id")
		if _, err := s.get(r, id); s.respond(w, err) {
			return
		}
		if r.table == "categories" {
			var products int
			if s.respond(w, s.db.QueryRow(`SELECT COUNT(*) FROM products WHERE data->>'category_id' = $1`, id).Scan(&products)) {
				return
			}
			if products > 0 {
				web.Error(w, http.StatusConflict, "the category still has products")
				return
			}
		}
		if _, err := s.db.Exec(`DELETE FROM `+r.table+` WHERE id = $1`, id); s.respond(w, err) {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func main() {
	db, err := sql.Open("pgx", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	s := &server{db: db}
	if err := s.migrate(); err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	for _, r := range resources {
		s.register(mux, r)
	}
	web.Serve(mux)
}
