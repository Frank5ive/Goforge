// Command orders is the reference solution for the orders task.
package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"math"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/Frank5ive/goforge/benchmark/reference/internal/web"
)

type item struct {
	SKU       string  `json:"sku"`
	Quantity  int     `json:"quantity"`
	UnitPrice float64 `json:"unit_price"`
}

type order struct {
	ID        string    `json:"id"`
	Customer  string    `json:"customer"`
	Items     []item    `json:"items"`
	Total     float64   `json:"total"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

var statuses = []string{"pending", "paid", "shipped", "cancelled"}

// transitions lists, for each action, the statuses it may start from and the
// status it moves to.
var transitions = map[string]struct {
	from []string
	to   string
}{
	"pay":    {[]string{"pending"}, "paid"},
	"ship":   {[]string{"paid"}, "shipped"},
	"cancel": {[]string{"pending", "paid"}, "cancelled"},
}

func validate(customer string, items []item) error {
	if strings.TrimSpace(customer) == "" {
		return errors.New("customer is required")
	}
	if len(items) == 0 {
		return errors.New("at least one item is required")
	}
	for _, it := range items {
		switch {
		case strings.TrimSpace(it.SKU) == "":
			return errors.New("every item needs a sku")
		case it.Quantity < 1:
			return errors.New("quantity must be 1 or more")
		case it.UnitPrice < 0:
			return errors.New("unit_price must be 0 or more")
		}
	}
	return nil
}

func total(items []item) float64 {
	var sum float64
	for _, it := range items {
		sum += float64(it.Quantity) * it.UnitPrice
	}
	return math.Round(sum*100) / 100
}

const columns = `id, customer, items, total, status, created_at, updated_at`

type scanner interface{ Scan(...any) error }

func scan(row scanner) (order, error) {
	var o order
	var items []byte
	if err := row.Scan(&o.ID, &o.Customer, &items, &o.Total, &o.Status, &o.CreatedAt, &o.UpdatedAt); err != nil {
		return order{}, err
	}
	o.CreatedAt, o.UpdatedAt = o.CreatedAt.UTC(), o.UpdatedAt.UTC()
	return o, json.Unmarshal(items, &o.Items)
}

func main() {
	db, err := sql.Open("pgx", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS orders (
		seq        BIGSERIAL PRIMARY KEY,
		id         TEXT NOT NULL UNIQUE,
		customer   TEXT NOT NULL,
		items      JSONB NOT NULL,
		total      DOUBLE PRECISION NOT NULL,
		status     TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL,
		updated_at TIMESTAMPTZ NOT NULL
	)`); err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /orders", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Customer string `json:"customer"`
			Items    []item `json:"items"`
		}
		if !web.Decode(w, r, &in) {
			return
		}
		if err := validate(in.Customer, in.Items); err != nil {
			web.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		now := time.Now().UTC()
		o := order{ID: web.NewID(), Customer: in.Customer, Items: in.Items, Total: total(in.Items), Status: "pending", CreatedAt: now, UpdatedAt: now}
		items, _ := json.Marshal(o.Items)
		if _, err := db.Exec(`INSERT INTO orders (`+columns+`) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			o.ID, o.Customer, items, o.Total, o.Status, o.CreatedAt, o.UpdatedAt); err != nil {
			web.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		web.JSON(w, http.StatusCreated, o)
	})
	mux.HandleFunc("GET /orders", func(w http.ResponseWriter, r *http.Request) {
		limit, offset, ok := web.Paging(w, r)
		if !ok {
			return
		}
		status := r.URL.Query().Get("status")
		if status != "" && !slices.Contains(statuses, status) {
			web.Error(w, http.StatusBadRequest, "unknown status")
			return
		}
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM orders WHERE $1 = '' OR status = $1`, status).Scan(&count); err != nil {
			web.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		rows, err := db.Query(`SELECT `+columns+` FROM orders WHERE $1 = '' OR status = $1 ORDER BY seq LIMIT $2 OFFSET $3`, status, limit, offset)
		if err != nil {
			web.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer rows.Close()
		items := []order{}
		for rows.Next() {
			o, err := scan(rows)
			if err != nil {
				web.Error(w, http.StatusInternalServerError, err.Error())
				return
			}
			items = append(items, o)
		}
		web.JSON(w, http.StatusOK, web.Page[order]{Items: items, Total: count, Limit: limit, Offset: offset})
	})
	get := func(w http.ResponseWriter, id string) (order, bool) {
		o, err := scan(db.QueryRow(`SELECT `+columns+` FROM orders WHERE id = $1`, id))
		switch {
		case errors.Is(err, sql.ErrNoRows):
			web.Error(w, http.StatusNotFound, "order not found")
			return order{}, false
		case err != nil:
			web.Error(w, http.StatusInternalServerError, err.Error())
			return order{}, false
		}
		return o, true
	}
	mux.HandleFunc("GET /orders/{id}", func(w http.ResponseWriter, r *http.Request) {
		if o, ok := get(w, r.PathValue("id")); ok {
			web.JSON(w, http.StatusOK, o)
		}
	})
	mux.HandleFunc("POST /orders/{id}/{action}", func(w http.ResponseWriter, r *http.Request) {
		t, known := transitions[r.PathValue("action")]
		if !known {
			web.Error(w, http.StatusNotFound, "unknown action")
			return
		}
		o, ok := get(w, r.PathValue("id"))
		if !ok {
			return
		}
		// Change the status only if it is still one the action starts from
		now := time.Now().UTC()
		res, err := db.Exec(`UPDATE orders SET status = $1, updated_at = $2 WHERE id = $3 AND status = ANY($4)`, t.to, now, o.ID, t.from)
		if err != nil {
			web.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			web.Error(w, http.StatusConflict, "a "+o.Status+" order can't be changed that way")
			return
		}
		o.Status, o.UpdatedAt = t.to, now
		web.JSON(w, http.StatusOK, o)
	})

	web.Serve(mux)
}
