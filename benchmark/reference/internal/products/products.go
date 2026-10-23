// Package products is the reference products API, over a pluggable store.
package products

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Frank5ive/goforge/benchmark/reference/internal/web"
)

// Product is a catalog entry.
type Product struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Price     float64   `json:"price"`
	InStock   bool      `json:"in_stock"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Store keeps products, oldest first.
type Store interface {
	List(limit, offset int) ([]Product, int, error)
	Get(id string) (Product, error)
	Create(p Product) error
	Update(p Product) error
	Delete(id string) error
}

// input is the client-editable part of a product; nil fields were not sent.
type input struct {
	Name    *string  `json:"name"`
	Price   *float64 `json:"price"`
	InStock *bool    `json:"in_stock"`
}

func validate(p Product) error {
	if n := utf8.RuneCountInString(strings.TrimSpace(p.Name)); n < 1 || n > 100 {
		return errors.New("name must be 1 to 100 characters")
	}
	if p.Price < 0 {
		return errors.New("price must be 0 or more")
	}
	return nil
}

// Register adds the product routes to mux.
func Register(mux *http.ServeMux, store Store) {
	mux.HandleFunc("POST /products", func(w http.ResponseWriter, r *http.Request) {
		var in input
		if !web.Decode(w, r, &in) {
			return
		}
		now := time.Now().UTC()
		p := Product{ID: web.NewID(), CreatedAt: now, UpdatedAt: now}
		replace(&p, in)
		if err := validate(p); err != nil {
			web.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		if err := store.Create(p); err != nil {
			web.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		web.JSON(w, http.StatusCreated, p)
	})
	mux.HandleFunc("GET /products", func(w http.ResponseWriter, r *http.Request) {
		limit, offset, ok := web.Paging(w, r)
		if !ok {
			return
		}
		items, total, err := store.List(limit, offset)
		if err != nil {
			web.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		web.JSON(w, http.StatusOK, web.Page[Product]{Items: items, Total: total, Limit: limit, Offset: offset})
	})
	mux.HandleFunc("GET /products/{id}", func(w http.ResponseWriter, r *http.Request) {
		p, err := store.Get(r.PathValue("id"))
		if respondErr(w, err) {
			return
		}
		web.JSON(w, http.StatusOK, p)
	})
	update := func(full bool) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			var in input
			if !web.Decode(w, r, &in) {
				return
			}
			p, err := store.Get(r.PathValue("id"))
			if respondErr(w, err) {
				return
			}
			if full {
				replace(&p, in)
			} else {
				patch(&p, in)
			}
			if err := validate(p); err != nil {
				web.Error(w, http.StatusUnprocessableEntity, err.Error())
				return
			}
			p.UpdatedAt = time.Now().UTC()
			if respondErr(w, store.Update(p)) {
				return
			}
			web.JSON(w, http.StatusOK, p)
		}
	}
	mux.HandleFunc("PUT /products/{id}", update(true))
	mux.HandleFunc("PATCH /products/{id}", update(false))
	mux.HandleFunc("DELETE /products/{id}", func(w http.ResponseWriter, r *http.Request) {
		if respondErr(w, store.Delete(r.PathValue("id"))) {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// replace sets every editable field, with defaults for those not sent.
func replace(p *Product, in input) {
	p.Name, p.Price, p.InStock = "", 0, false
	patch(p, in)
}

// patch sets the editable fields that were sent.
func patch(p *Product, in input) {
	if in.Name != nil {
		p.Name = *in.Name
	}
	if in.Price != nil {
		p.Price = *in.Price
	}
	if in.InStock != nil {
		p.InStock = *in.InStock
	}
}

func respondErr(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, web.ErrNotFound):
		web.Error(w, http.StatusNotFound, "product not found")
	default:
		web.Error(w, http.StatusInternalServerError, err.Error())
	}
	return true
}
