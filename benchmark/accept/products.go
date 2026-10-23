package accept

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
)

type productState struct {
	widget, gadget, long string // IDs, in creation order
}

func productChecks(Env) []check[productState] {
	return []check[productState]{
		{"health check responds 200", func(c *client, _ *productState) error {
			return c.health()
		}},
		{"create a product", func(c *client, s *productState) error {
			m, err := c.expect("POST", "/products", `{"name": "Widget", "price": 9.99, "in_stock": true}`, http.StatusCreated)
			if err != nil {
				return err
			}
			if s.widget, err = stored(m); err != nil {
				return err
			}
			return fields(m, map[string]any{"name": "Widget", "price": 9.99, "in_stock": true})
		}},
		{"price and in_stock default to 0 and false", func(c *client, s *productState) error {
			m, err := c.expect("POST", "/products", `{"name": "Gadget"}`, http.StatusCreated)
			if err != nil {
				return err
			}
			if s.gadget, err = stored(m); err != nil {
				return err
			}
			return fields(m, map[string]any{"name": "Gadget", "price": 0, "in_stock": false})
		}},
		{"a 100-character name is accepted", func(c *client, s *productState) error {
			m, err := c.expect("POST", "/products", `{"name": "`+strings.Repeat("n", 100)+`", "price": 1}`, http.StatusCreated)
			if err != nil {
				return err
			}
			s.long, err = stored(m)
			return err
		}},
		{"get a product by ID", func(c *client, s *productState) error {
			m, err := c.expect("GET", "/products/"+s.widget, "", http.StatusOK)
			if err != nil {
				return err
			}
			return fields(m, map[string]any{"id": s.widget, "name": "Widget", "price": 9.99, "in_stock": true})
		}},
		{"an unknown ID is a 404 with an error", func(c *client, _ *productState) error {
			return c.expectError("GET", "/products/does-not-exist", "", http.StatusNotFound)
		}},
		{"a missing name is a 422", func(c *client, _ *productState) error {
			return c.expectError("POST", "/products", `{"price": 1}`, http.StatusUnprocessableEntity)
		}},
		{"a blank name is a 422", func(c *client, _ *productState) error {
			return c.expectError("POST", "/products", `{"name": "   "}`, http.StatusUnprocessableEntity)
		}},
		{"a 101-character name is a 422", func(c *client, _ *productState) error {
			return c.expectError("POST", "/products", `{"name": "`+strings.Repeat("n", 101)+`"}`, http.StatusUnprocessableEntity)
		}},
		{"a negative price is a 422", func(c *client, _ *productState) error {
			return c.expectError("POST", "/products", `{"name": "Bad", "price": -1}`, http.StatusUnprocessableEntity)
		}},
		{"malformed JSON is a 400", func(c *client, _ *productState) error {
			return c.expectError("POST", "/products", `{"name": `, http.StatusBadRequest)
		}},
		{"invalid products are not stored", func(c *client, _ *productState) error {
			p, err := c.expectPage("/products")
			if err != nil {
				return err
			}
			if p.total != 3 {
				return fmt.Errorf("total = %d, want 3 (only the valid products)", p.total)
			}
			return nil
		}},
		{"list products oldest first with defaults", func(c *client, s *productState) error {
			p, err := c.expectPage("/products")
			if err != nil {
				return err
			}
			if want := []string{s.widget, s.gadget, s.long}; !equal(ids(p.items), want) {
				return fmt.Errorf("item IDs = %v, want %v", ids(p.items), want)
			}
			if p.limit != 20 || p.offset != 0 {
				return fmt.Errorf("limit, offset = %d, %d, want 20, 0", p.limit, p.offset)
			}
			return nil
		}},
		{"limit and offset page through the list", func(c *client, s *productState) error {
			p, err := c.expectPage("/products?limit=1&offset=1")
			if err != nil {
				return err
			}
			if !equal(ids(p.items), []string{s.gadget}) || p.total != 3 || p.limit != 1 || p.offset != 1 {
				return fmt.Errorf("got items %v, total %d, limit %d, offset %d; want [%s], 3, 1, 1", ids(p.items), p.total, p.limit, p.offset, s.gadget)
			}
			return nil
		}},
		{"a limit above 100 is treated as 100", func(c *client, _ *productState) error {
			p, err := c.expectPage("/products?limit=500")
			if err != nil {
				return err
			}
			if p.limit != 100 {
				return fmt.Errorf("limit = %d, want 100", p.limit)
			}
			return nil
		}},
		{"a negative limit is a 400", func(c *client, _ *productState) error {
			return c.expectError("GET", "/products?limit=-1", "", http.StatusBadRequest)
		}},
		{"a non-numeric offset is a 400", func(c *client, _ *productState) error {
			return c.expectError("GET", "/products?offset=abc", "", http.StatusBadRequest)
		}},
		{"replace a product with PUT", func(c *client, s *productState) error {
			m, err := c.expect("PUT", "/products/"+s.widget, `{"name": "Widget 2", "price": 5, "in_stock": false}`, http.StatusOK)
			if err != nil {
				return err
			}
			if err := fields(m, map[string]any{"id": s.widget, "name": "Widget 2", "price": 5, "in_stock": false}); err != nil {
				return err
			}
			m, err = c.expect("GET", "/products/"+s.widget, "", http.StatusOK)
			if err != nil {
				return err
			}
			return fields(m, map[string]any{"name": "Widget 2", "price": 5, "in_stock": false})
		}},
		{"PUT applies the same rules as POST", func(c *client, s *productState) error {
			return c.expectError("PUT", "/products/"+s.widget, `{"name": "", "price": 1}`, http.StatusUnprocessableEntity)
		}},
		{"PUT to an unknown ID is a 404", func(c *client, _ *productState) error {
			return c.expectError("PUT", "/products/does-not-exist", `{"name": "X", "price": 1}`, http.StatusNotFound)
		}},
		{"PATCH changes only the fields sent", func(c *client, s *productState) error {
			m, err := c.expect("PATCH", "/products/"+s.widget, `{"price": 7.5}`, http.StatusOK)
			if err != nil {
				return err
			}
			return fields(m, map[string]any{"name": "Widget 2", "price": 7.5, "in_stock": false})
		}},
		{"an invalid PATCH is a 422 and changes nothing", func(c *client, s *productState) error {
			if err := c.expectError("PATCH", "/products/"+s.widget, `{"price": -3}`, http.StatusUnprocessableEntity); err != nil {
				return err
			}
			m, err := c.expect("GET", "/products/"+s.widget, "", http.StatusOK)
			if err != nil {
				return err
			}
			return fields(m, map[string]any{"price": 7.5})
		}},
		{"PATCH to an unknown ID is a 404", func(c *client, _ *productState) error {
			return c.expectError("PATCH", "/products/does-not-exist", `{"price": 1}`, http.StatusNotFound)
		}},
		{"delete a product", func(c *client, s *productState) error {
			if _, err := c.expect("DELETE", "/products/"+s.gadget, "", http.StatusNoContent); err != nil {
				return err
			}
			if err := c.expectError("GET", "/products/"+s.gadget, "", http.StatusNotFound); err != nil {
				return err
			}
			p, err := c.expectPage("/products")
			if err != nil {
				return err
			}
			if p.total != 2 {
				return fmt.Errorf("total after delete = %d, want 2", p.total)
			}
			return nil
		}},
		{"deleting an unknown ID is a 404", func(c *client, s *productState) error {
			return c.expectError("DELETE", "/products/"+s.gadget, "", http.StatusNotFound)
		}},
	}
}

func productSQLiteChecks(env Env) []check[productState] {
	return append(productChecks(env),
		check[productState]{"products survive a restart", func(c *client, s *productState) error {
			if env.Restart == nil {
				return errors.New("the harness can't restart the service")
			}
			if err := env.Restart(); err != nil {
				return fmt.Errorf("restarting the service: %w", err)
			}
			p, err := c.expectPage("/products")
			if err != nil {
				return err
			}
			if want := []string{s.widget, s.long}; !equal(ids(p.items), want) || p.total != 2 {
				return fmt.Errorf("after a restart: items %v, total %d; want %v, 2", ids(p.items), p.total, want)
			}
			m, err := c.expect("GET", "/products/"+s.widget, "", http.StatusOK)
			if err != nil {
				return err
			}
			return fields(m, map[string]any{"name": "Widget 2", "price": 7.5, "in_stock": false})
		}},
		check[productState]{"data is stored in the SQLite file at DATABASE_URL", func(*client, *productState) error {
			data, err := os.ReadFile(env.DatabaseFile)
			if err != nil {
				return fmt.Errorf("reading DATABASE_URL's file: %w", err)
			}
			if !bytes.HasPrefix(data, []byte("SQLite format 3\x00")) {
				return errors.New("the file at DATABASE_URL is not a SQLite database")
			}
			return nil
		}},
	)
}
