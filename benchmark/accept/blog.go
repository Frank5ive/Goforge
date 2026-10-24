package accept

import (
	"fmt"
	"net/http"
)

type blogState struct {
	ada, bob    string // author IDs
	hello, news string // post IDs: Ada's, then Bob's
}

func blogChecks(Env) []check[blogState] {
	return []check[blogState]{
		{"health check responds 200", func(c *client, _ *blogState) error {
			return c.health()
		}},
		{"create an author", func(c *client, s *blogState) error {
			m, err := c.expect("POST", "/authors", `{"name": "Ada", "email": "ada@example.com"}`, http.StatusCreated)
			if err != nil {
				return err
			}
			if s.ada, err = stored(m); err != nil {
				return err
			}
			return fields(m, map[string]any{"name": "Ada", "email": "ada@example.com"})
		}},
		{"an email without @ is a 422", func(c *client, _ *blogState) error {
			return c.expectError("POST", "/authors", `{"name": "Eve", "email": "eve.example.com"}`, http.StatusUnprocessableEntity)
		}},
		{"an email with two @ is a 422", func(c *client, _ *blogState) error {
			return c.expectError("POST", "/authors", `{"name": "Eve", "email": "eve@ex@ample.com"}`, http.StatusUnprocessableEntity)
		}},
		{"an email with nothing before @ is a 422", func(c *client, _ *blogState) error {
			return c.expectError("POST", "/authors", `{"name": "Eve", "email": "@example.com"}`, http.StatusUnprocessableEntity)
		}},
		{"a blank author name is a 422", func(c *client, _ *blogState) error {
			return c.expectError("POST", "/authors", `{"name": "  ", "email": "eve@example.com"}`, http.StatusUnprocessableEntity)
		}},
		{"create a post, unpublished by default", func(c *client, s *blogState) error {
			m, err := c.expect("POST", "/posts", fmt.Sprintf(`{"author_id": %q, "title": "Hello", "body": "First post"}`, s.ada), http.StatusCreated)
			if err != nil {
				return err
			}
			if s.hello, err = stored(m); err != nil {
				return err
			}
			return fields(m, map[string]any{"author_id": s.ada, "title": "Hello", "body": "First post", "published": false})
		}},
		{"a post by an unknown author is a 422", func(c *client, _ *blogState) error {
			return c.expectError("POST", "/posts", `{"author_id": "does-not-exist", "title": "Hi", "body": "Text"}`, http.StatusUnprocessableEntity)
		}},
		{"a post without a title is a 422", func(c *client, s *blogState) error {
			return c.expectError("POST", "/posts", fmt.Sprintf(`{"author_id": %q, "title": " ", "body": "Text"}`, s.ada), http.StatusUnprocessableEntity)
		}},
		{"malformed JSON is a 400", func(c *client, _ *blogState) error {
			return c.expectError("POST", "/posts", `{"title": `, http.StatusBadRequest)
		}},
		{"get an author and a post by ID", func(c *client, s *blogState) error {
			m, err := c.expect("GET", "/authors/"+s.ada, "", http.StatusOK)
			if err != nil {
				return err
			}
			if err := fields(m, map[string]any{"id": s.ada, "name": "Ada"}); err != nil {
				return err
			}
			m, err = c.expect("GET", "/posts/"+s.hello, "", http.StatusOK)
			if err != nil {
				return err
			}
			return fields(m, map[string]any{"id": s.hello, "title": "Hello"})
		}},
		{"unknown IDs are a 404", func(c *client, _ *blogState) error {
			if err := c.expectError("GET", "/authors/does-not-exist", "", http.StatusNotFound); err != nil {
				return err
			}
			return c.expectError("GET", "/posts/does-not-exist", "", http.StatusNotFound)
		}},
		{"list an author's posts", func(c *client, s *blogState) error {
			m, err := c.expect("POST", "/authors", `{"name": "Bob", "email": "bob@example.com"}`, http.StatusCreated)
			if err != nil {
				return err
			}
			if s.bob, err = stored(m); err != nil {
				return err
			}
			m, err = c.expect("POST", "/posts", fmt.Sprintf(`{"author_id": %q, "title": "News", "body": "Second post", "published": true}`, s.bob), http.StatusCreated)
			if err != nil {
				return err
			}
			if s.news, err = stored(m); err != nil {
				return err
			}
			for author, post := range map[string]string{s.ada: s.hello, s.bob: s.news} {
				p, err := c.expectPage("/authors/" + author + "/posts")
				if err != nil {
					return err
				}
				if !equal(ids(p.items), []string{post}) || p.total != 1 {
					return fmt.Errorf("posts of %s = %v (total %d), want only %s", author, ids(p.items), p.total, post)
				}
			}
			return nil
		}},
		{"an unknown author's posts are a 404", func(c *client, _ *blogState) error {
			return c.expectError("GET", "/authors/does-not-exist/posts", "", http.StatusNotFound)
		}},
		{"list every post oldest first", func(c *client, s *blogState) error {
			p, err := c.expectPage("/posts")
			if err != nil {
				return err
			}
			if want := []string{s.hello, s.news}; !equal(ids(p.items), want) || p.total != 2 {
				return fmt.Errorf("posts = %v (total %d), want %v", ids(p.items), p.total, want)
			}
			return nil
		}},
		{"limit and offset page through authors", func(c *client, s *blogState) error {
			p, err := c.expectPage("/authors?limit=1&offset=1")
			if err != nil {
				return err
			}
			if !equal(ids(p.items), []string{s.bob}) || p.total != 2 || p.limit != 1 || p.offset != 1 {
				return fmt.Errorf("got items %v, total %d, limit %d, offset %d; want [%s], 2, 1, 1", ids(p.items), p.total, p.limit, p.offset, s.bob)
			}
			return nil
		}},
		{"an invalid limit is a 400", func(c *client, _ *blogState) error {
			return c.expectError("GET", "/posts?limit=ten", "", http.StatusBadRequest)
		}},
		{"deleting an author with posts is a 409 and keeps them", func(c *client, s *blogState) error {
			if err := c.expectError("DELETE", "/authors/"+s.ada, "", http.StatusConflict); err != nil {
				return err
			}
			_, err := c.expect("GET", "/authors/"+s.ada, "", http.StatusOK)
			return err
		}},
		{"replace a post with PUT", func(c *client, s *blogState) error {
			m, err := c.expect("PUT", "/posts/"+s.hello, fmt.Sprintf(`{"author_id": %q, "title": "Hello again", "body": "Edited", "published": true}`, s.ada), http.StatusOK)
			if err != nil {
				return err
			}
			return fields(m, map[string]any{"title": "Hello again", "body": "Edited", "published": true})
		}},
		{"PUT a post to an unknown author is a 422", func(c *client, s *blogState) error {
			return c.expectError("PUT", "/posts/"+s.hello, `{"author_id": "does-not-exist", "title": "T", "body": "B"}`, http.StatusUnprocessableEntity)
		}},
		{"replace an author with PUT, applying the rules", func(c *client, s *blogState) error {
			if err := c.expectError("PUT", "/authors/"+s.bob, `{"name": "Bob", "email": "not-an-email"}`, http.StatusUnprocessableEntity); err != nil {
				return err
			}
			m, err := c.expect("PUT", "/authors/"+s.bob, `{"name": "Robert", "email": "robert@example.com"}`, http.StatusOK)
			if err != nil {
				return err
			}
			return fields(m, map[string]any{"name": "Robert", "email": "robert@example.com"})
		}},
		{"an author can be deleted once their posts are gone", func(c *client, s *blogState) error {
			if _, err := c.expect("DELETE", "/posts/"+s.hello, "", http.StatusNoContent); err != nil {
				return err
			}
			if _, err := c.expect("DELETE", "/authors/"+s.ada, "", http.StatusNoContent); err != nil {
				return err
			}
			return c.expectError("GET", "/authors/"+s.ada, "", http.StatusNotFound)
		}},
		{"deleting an unknown post is a 404", func(c *client, s *blogState) error {
			return c.expectError("DELETE", "/posts/"+s.hello, "", http.StatusNotFound)
		}},
	}
}
