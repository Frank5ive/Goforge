package accept

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// storeResource describes one store resource for the checks every resource
// gets. In bodies, {cat} stands for the ID of an existing category.
type storeResource struct {
	name, path string
	// create is a body with every field, and created the fields it must return
	create  string
	created map[string]any
	// minimal is a body with only the required fields, and defaults the values
	// the optional fields must get
	minimal  string
	defaults map[string]any
	// invalid bodies break a rule each
	invalid []string
	// replace is a PUT body, replaced the fields it must return
	replace  string
	replaced map[string]any
	// patch is a PATCH body, patched the fields after it (including unchanged
	// ones), and badPatch a PATCH that breaks a rule
	patch    string
	patched  map[string]any
	badPatch string
}

var storeResources = []storeResource{
	{
		name: "categories", path: "/categories",
		create:   `{"name": "Tools", "description": "Hand tools"}`,
		created:  map[string]any{"name": "Tools", "description": "Hand tools"},
		minimal:  `{"name": "Garden"}`,
		defaults: map[string]any{"name": "Garden", "description": ""},
		invalid:  []string{`{}`, `{"name": "  "}`},
		replace:  `{"name": "Power tools", "description": "Electric"}`,
		replaced: map[string]any{"name": "Power tools", "description": "Electric"},
		patch:    `{"description": "Drills and saws"}`,
		patched:  map[string]any{"name": "Power tools", "description": "Drills and saws"},
		badPatch: `{"name": ""}`,
	},
	{
		name: "products", path: "/products",
		create:   `{"name": "Widget", "sku": "W-1", "price": 9.99, "in_stock": true, "category_id": "{cat}"}`,
		created:  map[string]any{"name": "Widget", "sku": "W-1", "price": 9.99, "in_stock": true},
		minimal:  `{"name": "Bolt", "sku": "B-1", "category_id": "{cat}"}`,
		defaults: map[string]any{"name": "Bolt", "price": 0, "in_stock": false},
		invalid: []string{
			`{"sku": "X-1", "category_id": "{cat}"}`,
			`{"name": " ", "sku": "X-1", "category_id": "{cat}"}`,
			`{"name": "X", "sku": "", "category_id": "{cat}"}`,
			`{"name": "X", "sku": "X-1", "price": -1, "category_id": "{cat}"}`,
			`{"name": "X", "sku": "X-1"}`,
		},
		replace:  `{"name": "Widget Pro", "sku": "W-2", "price": 12.5, "in_stock": true, "category_id": "{cat}"}`,
		replaced: map[string]any{"name": "Widget Pro", "sku": "W-2", "price": 12.5, "in_stock": true},
		patch:    `{"price": 15}`,
		patched:  map[string]any{"name": "Widget Pro", "sku": "W-2", "price": 15, "in_stock": true},
		badPatch: `{"price": -1}`,
	},
	{
		name: "customers", path: "/customers",
		create:   `{"name": "Ada", "email": "ada@example.com"}`,
		created:  map[string]any{"name": "Ada", "email": "ada@example.com"},
		minimal:  `{"name": "Bob", "email": "bob@example.com"}`,
		defaults: map[string]any{"name": "Bob", "email": "bob@example.com"},
		invalid: []string{
			`{"name": "Eve"}`,
			`{"name": "Eve", "email": "eve.example.com"}`,
			`{"name": "Eve", "email": "eve@ex@ample.com"}`,
			`{"name": "", "email": "eve@example.com"}`,
		},
		replace:  `{"name": "Ada L", "email": "ada@lovelace.org"}`,
		replaced: map[string]any{"name": "Ada L", "email": "ada@lovelace.org"},
		patch:    `{"name": "Ada Lovelace"}`,
		patched:  map[string]any{"name": "Ada Lovelace", "email": "ada@lovelace.org"},
		badPatch: `{"email": "nope"}`,
	},
	{
		name: "suppliers", path: "/suppliers",
		create:   `{"name": "Acme", "country": "MW", "phone": "+265 1 234"}`,
		created:  map[string]any{"name": "Acme", "country": "MW", "phone": "+265 1 234"},
		minimal:  `{"name": "Globex", "country": "ZA"}`,
		defaults: map[string]any{"name": "Globex", "country": "ZA", "phone": ""},
		invalid:  []string{`{"name": "Initech"}`, `{"name": " ", "country": "MW"}`},
		replace:  `{"name": "Acme Ltd", "country": "MW", "phone": ""}`,
		replaced: map[string]any{"name": "Acme Ltd", "country": "MW", "phone": ""},
		patch:    `{"phone": "+265 999"}`,
		patched:  map[string]any{"name": "Acme Ltd", "country": "MW", "phone": "+265 999"},
		badPatch: `{"country": " "}`,
	},
	{
		name: "warehouses", path: "/warehouses",
		create:   `{"name": "Main", "city": "Blantyre", "capacity": 500}`,
		created:  map[string]any{"name": "Main", "city": "Blantyre", "capacity": 500},
		minimal:  `{"name": "Annex", "city": "Lilongwe"}`,
		defaults: map[string]any{"name": "Annex", "city": "Lilongwe", "capacity": 0},
		invalid: []string{
			`{"name": "Depot"}`,
			`{"name": "Depot", "city": "Zomba", "capacity": -1}`,
			`{"name": "", "city": "Zomba"}`,
		},
		replace:  `{"name": "Main DC", "city": "Blantyre", "capacity": 800}`,
		replaced: map[string]any{"name": "Main DC", "city": "Blantyre", "capacity": 800},
		patch:    `{"capacity": 900}`,
		patched:  map[string]any{"name": "Main DC", "city": "Blantyre", "capacity": 900},
		badPatch: `{"capacity": -5}`,
	},
}

type storeState struct {
	// ids holds each resource's item IDs: the full one, then the minimal one
	ids map[string][]string
}

// body fills in the category placeholder.
func (s *storeState) body(b string) string {
	cat := ""
	if ids := s.ids["categories"]; len(ids) > 0 {
		cat = ids[0]
	}
	return strings.ReplaceAll(b, "{cat}", cat)
}

func (s *storeState) first(resource string) (string, error) {
	if ids := s.ids[resource]; len(ids) > 0 {
		return ids[0], nil
	}
	return "", fmt.Errorf("no %s were created", resource)
}

func storeChecks(env Env) []check[storeState] {
	checks := []check[storeState]{
		{"health check responds 200", func(c *client, s *storeState) error {
			s.ids = map[string][]string{}
			return c.health()
		}},
	}
	for _, r := range storeResources {
		checks = append(checks, resourceChecks(r)...)
	}
	return append(checks,
		check[storeState]{"a product in an unknown category is a 422", func(c *client, _ *storeState) error {
			return c.expectError("POST", "/products", `{"name": "Ghost", "sku": "G-1", "category_id": "does-not-exist"}`, http.StatusUnprocessableEntity)
		}},
		check[storeState]{"deleting a category with products is a 409 and keeps it", func(c *client, s *storeState) error {
			cat, err := s.first("categories")
			if err != nil {
				return err
			}
			if err := c.expectError("DELETE", "/categories/"+cat, "", http.StatusConflict); err != nil {
				return err
			}
			_, err = c.expect("GET", "/categories/"+cat, "", http.StatusOK)
			return err
		}},
		check[storeState]{"malformed JSON is a 400", func(c *client, _ *storeState) error {
			return c.expectError("POST", "/customers", `{"name": `, http.StatusBadRequest)
		}},
		check[storeState]{"invalid paging is a 400", func(c *client, _ *storeState) error {
			if err := c.expectError("GET", "/suppliers?limit=-1", "", http.StatusBadRequest); err != nil {
				return err
			}
			return c.expectError("GET", "/warehouses?offset=abc", "", http.StatusBadRequest)
		}},
		check[storeState]{"everything survives a restart", func(c *client, s *storeState) error {
			if env.Restart == nil {
				return errors.New("the harness can't restart the service")
			}
			if err := env.Restart(); err != nil {
				return fmt.Errorf("restarting the service: %w", err)
			}
			for _, r := range storeResources {
				id, err := s.first(r.name)
				if err != nil {
					return err
				}
				m, err := c.expect("GET", r.path+"/"+id, "", http.StatusOK)
				if err != nil {
					return fmt.Errorf("after a restart: %w", err)
				}
				if err := fields(m, r.patched); err != nil {
					return fmt.Errorf("after a restart, %s: %w", r.name, err)
				}
				p, err := c.expectPage(r.path)
				if err != nil {
					return err
				}
				if p.total != 1 {
					return fmt.Errorf("after a restart, %s total = %d, want 1", r.name, p.total)
				}
			}
			return nil
		}},
	)
}

// resourceChecks are the checks every store resource gets: create, defaults,
// get, validation, list and paging, PUT, PATCH, and delete.
func resourceChecks(r storeResource) []check[storeState] {
	return []check[storeState]{
		{r.name + ": create one", func(c *client, s *storeState) error {
			m, err := c.expect("POST", r.path, s.body(r.create), http.StatusCreated)
			if err != nil {
				return err
			}
			id, err := stored(m)
			if err != nil {
				return err
			}
			s.ids[r.name] = []string{id}
			return fields(m, r.created)
		}},
		{r.name + ": optional fields get their defaults", func(c *client, s *storeState) error {
			m, err := c.expect("POST", r.path, s.body(r.minimal), http.StatusCreated)
			if err != nil {
				return err
			}
			id, err := stored(m)
			if err != nil {
				return err
			}
			s.ids[r.name] = append(s.ids[r.name], id)
			return fields(m, r.defaults)
		}},
		{r.name + ": get one by ID, and 404 for an unknown ID", func(c *client, s *storeState) error {
			id, err := s.first(r.name)
			if err != nil {
				return err
			}
			m, err := c.expect("GET", r.path+"/"+id, "", http.StatusOK)
			if err != nil {
				return err
			}
			if err := fields(m, r.created); err != nil {
				return err
			}
			return c.expectError("GET", r.path+"/does-not-exist", "", http.StatusNotFound)
		}},
		{r.name + ": data that breaks a rule is a 422", func(c *client, s *storeState) error {
			for _, body := range r.invalid {
				if err := c.expectError("POST", r.path, s.body(body), http.StatusUnprocessableEntity); err != nil {
					return err
				}
			}
			return nil
		}},
		{r.name + ": list oldest first, with paging", func(c *client, s *storeState) error {
			p, err := c.expectPage(r.path)
			if err != nil {
				return err
			}
			if want := s.ids[r.name]; !equal(ids(p.items), want) || p.total != len(want) {
				return fmt.Errorf("items %v (total %d), want %v", ids(p.items), p.total, want)
			}
			p, err = c.expectPage(r.path + "?limit=1&offset=1")
			if err != nil {
				return err
			}
			if want := s.ids[r.name][1:]; !equal(ids(p.items), want) || p.limit != 1 || p.offset != 1 {
				return fmt.Errorf("limit=1&offset=1: items %v, limit %d, offset %d; want %v, 1, 1", ids(p.items), p.limit, p.offset, want)
			}
			return nil
		}},
		{r.name + ": PUT replaces an item, and 404 for an unknown ID", func(c *client, s *storeState) error {
			id, err := s.first(r.name)
			if err != nil {
				return err
			}
			m, err := c.expect("PUT", r.path+"/"+id, s.body(r.replace), http.StatusOK)
			if err != nil {
				return err
			}
			if err := fields(m, r.replaced); err != nil {
				return err
			}
			return c.expectError("PUT", r.path+"/does-not-exist", s.body(r.replace), http.StatusNotFound)
		}},
		{r.name + ": PATCH changes only the fields sent, and an invalid PATCH changes nothing", func(c *client, s *storeState) error {
			id, err := s.first(r.name)
			if err != nil {
				return err
			}
			m, err := c.expect("PATCH", r.path+"/"+id, s.body(r.patch), http.StatusOK)
			if err != nil {
				return err
			}
			if err := fields(m, r.patched); err != nil {
				return err
			}
			if err := c.expectError("PATCH", r.path+"/"+id, s.body(r.badPatch), http.StatusUnprocessableEntity); err != nil {
				return err
			}
			m, err = c.expect("GET", r.path+"/"+id, "", http.StatusOK)
			if err != nil {
				return err
			}
			return fields(m, r.patched)
		}},
		{r.name + ": delete an item, then 404", func(c *client, s *storeState) error {
			if len(s.ids[r.name]) < 2 {
				return fmt.Errorf("no second %s to delete", r.name)
			}
			id := s.ids[r.name][1]
			if _, err := c.expect("DELETE", r.path+"/"+id, "", http.StatusNoContent); err != nil {
				return err
			}
			s.ids[r.name] = s.ids[r.name][:1]
			if err := c.expectError("GET", r.path+"/"+id, "", http.StatusNotFound); err != nil {
				return err
			}
			return c.expectError("DELETE", r.path+"/"+id, "", http.StatusNotFound)
		}},
	}
}
