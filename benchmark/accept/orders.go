package accept

import (
	"errors"
	"fmt"
	"net/http"
)

type orderState struct {
	first, second string // order IDs
}

const firstOrder = `{"customer": "Ada", "items": [
	{"sku": "A1", "quantity": 2, "unit_price": 3.10},
	{"sku": "B2", "quantity": 1, "unit_price": 0.15}]}`

func orderChecks(env Env) []check[orderState] {
	return []check[orderState]{
		{"health check responds 200", func(c *client, _ *orderState) error {
			return c.health()
		}},
		{"create an order with a computed total", func(c *client, s *orderState) error {
			m, err := c.expect("POST", "/orders", firstOrder, http.StatusCreated)
			if err != nil {
				return err
			}
			if s.first, err = stored(m); err != nil {
				return err
			}
			if err := fields(m, map[string]any{"customer": "Ada", "total": 6.35, "status": "pending"}); err != nil {
				return err
			}
			items, ok := m["items"].([]any)
			if !ok || len(items) != 2 {
				return fmt.Errorf("items = %v, want the 2 items sent", m["items"])
			}
			item, _ := items[0].(map[string]any)
			return fields(item, map[string]any{"sku": "A1", "quantity": 2, "unit_price": 3.10})
		}},
		{"the total is rounded to 2 decimal places", func(c *client, _ *orderState) error {
			m, err := c.expect("POST", "/orders", `{"customer": "Bob", "items": [{"sku": "X", "quantity": 3, "unit_price": 0.333}]}`, http.StatusCreated)
			if err != nil {
				return err
			}
			return fields(m, map[string]any{"total": 1.0})
		}},
		{"the client can't set the status or total", func(c *client, _ *orderState) error {
			r, err := c.do("POST", "/orders", `{"customer": "Eve", "status": "shipped", "total": 999, "items": [{"sku": "X", "quantity": 1, "unit_price": 2}]}`)
			if err != nil {
				return err
			}
			if r.status != http.StatusCreated {
				return nil // rejecting the fields also keeps the client from setting them
			}
			m, err := r.json()
			if err != nil {
				return err
			}
			return fields(m, map[string]any{"status": "pending", "total": 2})
		}},
		{"an order without items is a 422", func(c *client, _ *orderState) error {
			if err := c.expectError("POST", "/orders", `{"customer": "Ada"}`, http.StatusUnprocessableEntity); err != nil {
				return err
			}
			return c.expectError("POST", "/orders", `{"customer": "Ada", "items": []}`, http.StatusUnprocessableEntity)
		}},
		{"a quantity below 1 is a 422", func(c *client, _ *orderState) error {
			return c.expectError("POST", "/orders", `{"customer": "Ada", "items": [{"sku": "X", "quantity": 0, "unit_price": 1}]}`, http.StatusUnprocessableEntity)
		}},
		{"a negative unit price is a 422", func(c *client, _ *orderState) error {
			return c.expectError("POST", "/orders", `{"customer": "Ada", "items": [{"sku": "X", "quantity": 1, "unit_price": -1}]}`, http.StatusUnprocessableEntity)
		}},
		{"a blank SKU or customer is a 422", func(c *client, _ *orderState) error {
			if err := c.expectError("POST", "/orders", `{"customer": "Ada", "items": [{"sku": " ", "quantity": 1, "unit_price": 1}]}`, http.StatusUnprocessableEntity); err != nil {
				return err
			}
			return c.expectError("POST", "/orders", `{"customer": "", "items": [{"sku": "X", "quantity": 1, "unit_price": 1}]}`, http.StatusUnprocessableEntity)
		}},
		{"malformed JSON is a 400", func(c *client, _ *orderState) error {
			return c.expectError("POST", "/orders", `{"customer": `, http.StatusBadRequest)
		}},
		{"get an order by ID", func(c *client, s *orderState) error {
			m, err := c.expect("GET", "/orders/"+s.first, "", http.StatusOK)
			if err != nil {
				return err
			}
			return fields(m, map[string]any{"id": s.first, "customer": "Ada", "total": 6.35, "status": "pending"})
		}},
		{"an unknown ID is a 404", func(c *client, _ *orderState) error {
			if err := c.expectError("GET", "/orders/does-not-exist", "", http.StatusNotFound); err != nil {
				return err
			}
			return c.expectError("POST", "/orders/does-not-exist/pay", "", http.StatusNotFound)
		}},
		{"a pending order can't be shipped", func(c *client, s *orderState) error {
			return c.expectError("POST", "/orders/"+s.first+"/ship", "", http.StatusConflict)
		}},
		{"pay, then ship an order", func(c *client, s *orderState) error {
			m, err := c.expect("POST", "/orders/"+s.first+"/pay", "", http.StatusOK)
			if err != nil {
				return err
			}
			if err := fields(m, map[string]any{"status": "paid"}); err != nil {
				return err
			}
			// Only a pending order can be paid
			if err := c.expectError("POST", "/orders/"+s.first+"/pay", "", http.StatusConflict); err != nil {
				return err
			}
			m, err = c.expect("POST", "/orders/"+s.first+"/ship", "", http.StatusOK)
			if err != nil {
				return err
			}
			return fields(m, map[string]any{"status": "shipped", "total": 6.35})
		}},
		{"a disallowed change is a 409 and leaves the order as it was", func(c *client, s *orderState) error {
			if err := c.expectError("POST", "/orders/"+s.first+"/pay", "", http.StatusConflict); err != nil {
				return err
			}
			if err := c.expectError("POST", "/orders/"+s.first+"/cancel", "", http.StatusConflict); err != nil {
				return err
			}
			m, err := c.expect("GET", "/orders/"+s.first, "", http.StatusOK)
			if err != nil {
				return err
			}
			return fields(m, map[string]any{"status": "shipped"})
		}},
		{"cancel a pending order, which then can't be paid", func(c *client, s *orderState) error {
			m, err := c.expect("POST", "/orders", `{"customer": "Cy", "items": [{"sku": "Z", "quantity": 1, "unit_price": 10}]}`, http.StatusCreated)
			if err != nil {
				return err
			}
			if s.second, err = stored(m); err != nil {
				return err
			}
			m, err = c.expect("POST", "/orders/"+s.second+"/cancel", "", http.StatusOK)
			if err != nil {
				return err
			}
			if err := fields(m, map[string]any{"status": "cancelled"}); err != nil {
				return err
			}
			return c.expectError("POST", "/orders/"+s.second+"/pay", "", http.StatusConflict)
		}},
		{"filter orders by status", func(c *client, s *orderState) error {
			for status, want := range map[string][]string{"shipped": {s.first}, "cancelled": {s.second}} {
				p, err := c.expectPage("/orders?status=" + status)
				if err != nil {
					return err
				}
				if !equal(ids(p.items), want) || p.total != 1 {
					return fmt.Errorf("status=%s: %v (total %d), want %v", status, ids(p.items), p.total, want)
				}
			}
			return nil
		}},
		{"an unknown status filter is a 400", func(c *client, _ *orderState) error {
			return c.expectError("GET", "/orders?status=lost", "", http.StatusBadRequest)
		}},
		{"list orders oldest first, with paging", func(c *client, s *orderState) error {
			p, err := c.expectPage("/orders")
			if err != nil {
				return err
			}
			// first, Bob's rounding order, possibly Eve's, then second
			if len(p.items) < 3 || ids(p.items)[0] != s.first || ids(p.items)[len(p.items)-1] != s.second || p.total != len(p.items) {
				return fmt.Errorf("orders = %v (total %d), want %s first and %s last", ids(p.items), p.total, s.first, s.second)
			}
			p2, err := c.expectPage("/orders?limit=1&offset=1")
			if err != nil {
				return err
			}
			if len(p2.items) != 1 || ids(p2.items)[0] != ids(p.items)[1] || p2.limit != 1 || p2.offset != 1 {
				return fmt.Errorf("limit=1&offset=1: %v (limit %d, offset %d), want [%s]", ids(p2.items), p2.limit, p2.offset, ids(p.items)[1])
			}
			return nil
		}},
		{"a negative offset is a 400", func(c *client, _ *orderState) error {
			return c.expectError("GET", "/orders?offset=-5", "", http.StatusBadRequest)
		}},
		{"orders survive a restart", func(c *client, s *orderState) error {
			if env.Restart == nil {
				return errors.New("the harness can't restart the service")
			}
			if err := env.Restart(); err != nil {
				return fmt.Errorf("restarting the service: %w", err)
			}
			m, err := c.expect("GET", "/orders/"+s.first, "", http.StatusOK)
			if err != nil {
				return err
			}
			if err := fields(m, map[string]any{"customer": "Ada", "total": 6.35, "status": "shipped"}); err != nil {
				return err
			}
			items, ok := m["items"].([]any)
			if !ok || len(items) != 2 {
				return fmt.Errorf("after a restart, items = %v, want 2", m["items"])
			}
			return nil
		}},
	}
}
