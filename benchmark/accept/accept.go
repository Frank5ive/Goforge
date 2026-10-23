// Package accept holds the hidden acceptance checks for the benchmark tasks.
// They test a running service over HTTP, so they judge the behavior the spec
// asks for, not how the code was written.
package accept

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

// Env is the running service under test.
type Env struct {
	// BaseURL is where the service listens, e.g. "http://127.0.0.1:18080".
	BaseURL string
	// Restart stops the service and starts it again with the same storage. It is
	// nil for tasks whose data may be kept in memory.
	Restart func() error
	// DatabaseFile is the SQLite database file, for tasks that use one.
	DatabaseFile string
}

// Result is the outcome of one named check.
type Result struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"`
}

// Suite runs a task's checks against env and returns their results.
type Suite func(env Env) []Result

// Suites are the checks for each task, by task name.
var Suites = map[string]Suite{
	"products":        func(env Env) []Result { return runChecks(env, productChecks) },
	"products-sqlite": func(env Env) []Result { return runChecks(env, productSQLiteChecks) },
	"blog":            func(env Env) []Result { return runChecks(env, blogChecks) },
	"orders":          func(env Env) []Result { return runChecks(env, orderChecks) },
}

// Count returns the number of checks for task, or 0 for an unknown task.
func Count(task string) int {
	switch task {
	case "products":
		return len(productChecks(Env{}))
	case "products-sqlite":
		return len(productSQLiteChecks(Env{}))
	case "blog":
		return len(blogChecks(Env{}))
	case "orders":
		return len(orderChecks(Env{}))
	}
	return 0
}

// check is one named check. Checks run in order and share state through the
// fields of the task's state struct, so later checks build on earlier ones.
type check[S any] struct {
	name string
	run  func(c *client, s *S) error
}

func runChecks[S any](env Env, checks func(Env) []check[S]) []Result {
	c := &client{base: strings.TrimSuffix(env.BaseURL, "/"), http: &http.Client{Timeout: 10 * time.Second}}
	var state S
	var results []Result
	for _, ch := range checks(env) {
		err := safely(func() error { return ch.run(c, &state) })
		r := Result{Name: ch.name, Passed: err == nil}
		if err != nil {
			r.Detail = err.Error()
		}
		results = append(results, r)
	}
	return results
}

// safely runs fn, turning a panic (such as a type assertion on an unexpected
// response) into a failed check.
func safely(fn func() error) (err error) {
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf("unexpected response: %v", v)
		}
	}()
	return fn()
}

// client sends requests to the service.
type client struct {
	base string
	http *http.Client
}

// response is a service's answer.
type response struct {
	status int
	body   []byte
}

// json decodes the body as a JSON object.
func (r response) json() (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal(r.body, &m); err != nil {
		return nil, fmt.Errorf("body is not a JSON object: %v: %.200s", err, r.body)
	}
	return m, nil
}

// do sends body (a JSON string, or "" for none) and returns the response.
func (c *client) do(method, path, body string) (response, error) {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		return response{}, err
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return response{}, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return response{}, err
	}
	return response{status: resp.StatusCode, body: data}, nil
}

// health requires GET /health to respond 200.
func (c *client) health() error {
	r, err := c.do("GET", "/health", "")
	if err != nil {
		return err
	}
	if r.status != http.StatusOK {
		return fmt.Errorf("GET /health = %d, want 200", r.status)
	}
	return nil
}

// expect sends the request and requires the status, returning the decoded JSON
// object for statuses that have a body.
func (c *client) expect(method, path, body string, status int) (map[string]any, error) {
	r, err := c.do(method, path, body)
	if err != nil {
		return nil, err
	}
	if r.status != status {
		return nil, fmt.Errorf("%s %s = %d, want %d: %.300s", method, path, r.status, status, bytes.TrimSpace(r.body))
	}
	if status == http.StatusNoContent {
		return nil, nil
	}
	return r.json()
}

// expectError requires the status and a JSON body with a non-empty "error" string.
func (c *client) expectError(method, path, body string, status int) error {
	m, err := c.expect(method, path, body, status)
	if err != nil {
		return err
	}
	if s, _ := m["error"].(string); s == "" {
		return fmt.Errorf("%s %s: body has no \"error\" message: %v", method, path, m)
	}
	return nil
}

// page is a decoded list response.
type page struct {
	items                []map[string]any
	total, limit, offset int
}

// expectPage requires a 200 list response with the page fields.
func (c *client) expectPage(path string) (page, error) {
	m, err := c.expect("GET", path, "", http.StatusOK)
	if err != nil {
		return page{}, err
	}
	raw, ok := m["items"].([]any)
	if !ok {
		return page{}, fmt.Errorf("GET %s: \"items\" is not a list: %v", path, m)
	}
	var p page
	for _, item := range raw {
		obj, ok := item.(map[string]any)
		if !ok {
			return page{}, fmt.Errorf("GET %s: an item is not an object", path)
		}
		p.items = append(p.items, obj)
	}
	for field, dst := range map[string]*int{"total": &p.total, "limit": &p.limit, "offset": &p.offset} {
		n, ok := m[field].(float64)
		if !ok {
			return page{}, fmt.Errorf("GET %s: %q is not a number: %v", path, field, m)
		}
		*dst = int(n)
	}
	return p, nil
}

// fields requires obj to have the given values. Numbers compare as float64.
func fields(obj map[string]any, want map[string]any) error {
	for key, w := range want {
		got, ok := obj[key]
		if !ok {
			return fmt.Errorf("field %q is missing: %v", key, obj)
		}
		if wf, isNum := toFloat(w); isNum {
			gf, gIsNum := got.(float64)
			if !gIsNum || math.Abs(gf-wf) > 1e-9 {
				return fmt.Errorf("%s = %v, want %v", key, got, w)
			}
			continue
		}
		if got != w {
			return fmt.Errorf("%s = %#v, want %#v", key, got, w)
		}
	}
	return nil
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	}
	return 0, false
}

// stored requires the fields every stored item has: a non-empty string id and
// RFC 3339 created_at and updated_at.
func stored(obj map[string]any) (string, error) {
	id, _ := obj["id"].(string)
	if id == "" {
		return "", fmt.Errorf("\"id\" is missing or not a string: %v", obj)
	}
	for _, field := range []string{"created_at", "updated_at"} {
		s, _ := obj[field].(string)
		if _, err := time.Parse(time.RFC3339, s); err != nil {
			return "", fmt.Errorf("%q is not an RFC 3339 timestamp: %v", field, obj[field])
		}
	}
	return id, nil
}

// ids returns the "id" of each item, in order.
func ids(items []map[string]any) []string {
	var out []string
	for _, item := range items {
		id, _ := item["id"].(string)
		out = append(out, id)
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
