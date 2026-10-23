// Command blog is the reference solution for the blog task.
package main

import (
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Frank5ive/goforge/benchmark/reference/internal/web"
)

type author struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type post struct {
	ID        string    `json:"id"`
	AuthorID  string    `json:"author_id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Published bool      `json:"published"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type blog struct {
	mu      sync.Mutex
	authors []author
	posts   []post
}

func (b *blog) authorIndex(id string) int {
	return slices.IndexFunc(b.authors, func(a author) bool { return a.ID == id })
}

func (b *blog) postIndex(id string) int {
	return slices.IndexFunc(b.posts, func(p post) bool { return p.ID == id })
}

func validEmail(email string) bool {
	user, domain, ok := strings.Cut(email, "@")
	return ok && user != "" && domain != "" && !strings.Contains(domain, "@")
}

func (b *blog) checkAuthor(a author) error {
	if strings.TrimSpace(a.Name) == "" {
		return errors.New("name is required")
	}
	if !validEmail(a.Email) {
		return errors.New("email is not valid")
	}
	return nil
}

func (b *blog) checkPost(p post) error {
	switch {
	case b.authorIndex(p.AuthorID) < 0:
		return errors.New("author_id is not an existing author")
	case strings.TrimSpace(p.Title) == "":
		return errors.New("title is required")
	case strings.TrimSpace(p.Body) == "":
		return errors.New("body is required")
	}
	return nil
}

func main() {
	b := &blog{}
	mux := http.NewServeMux()

	mux.HandleFunc("POST /authors", func(w http.ResponseWriter, r *http.Request) {
		var a author
		if !web.Decode(w, r, &a) {
			return
		}
		b.mu.Lock()
		defer b.mu.Unlock()
		if err := b.checkAuthor(a); err != nil {
			web.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		a.ID, a.CreatedAt = web.NewID(), time.Now().UTC()
		a.UpdatedAt = a.CreatedAt
		b.authors = append(b.authors, a)
		web.JSON(w, http.StatusCreated, a)
	})
	mux.HandleFunc("GET /authors", func(w http.ResponseWriter, r *http.Request) {
		limit, offset, ok := web.Paging(w, r)
		if !ok {
			return
		}
		b.mu.Lock()
		defer b.mu.Unlock()
		web.JSON(w, http.StatusOK, web.Page[author]{Items: web.Window(b.authors, limit, offset), Total: len(b.authors), Limit: limit, Offset: offset})
	})
	mux.HandleFunc("GET /authors/{id}", func(w http.ResponseWriter, r *http.Request) {
		b.mu.Lock()
		defer b.mu.Unlock()
		i := b.authorIndex(r.PathValue("id"))
		if i < 0 {
			web.Error(w, http.StatusNotFound, "author not found")
			return
		}
		web.JSON(w, http.StatusOK, b.authors[i])
	})
	mux.HandleFunc("PUT /authors/{id}", func(w http.ResponseWriter, r *http.Request) {
		var in author
		if !web.Decode(w, r, &in) {
			return
		}
		b.mu.Lock()
		defer b.mu.Unlock()
		i := b.authorIndex(r.PathValue("id"))
		if i < 0 {
			web.Error(w, http.StatusNotFound, "author not found")
			return
		}
		if err := b.checkAuthor(in); err != nil {
			web.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		a := &b.authors[i]
		a.Name, a.Email, a.UpdatedAt = in.Name, in.Email, time.Now().UTC()
		web.JSON(w, http.StatusOK, a)
	})
	mux.HandleFunc("DELETE /authors/{id}", func(w http.ResponseWriter, r *http.Request) {
		b.mu.Lock()
		defer b.mu.Unlock()
		id := r.PathValue("id")
		i := b.authorIndex(id)
		if i < 0 {
			web.Error(w, http.StatusNotFound, "author not found")
			return
		}
		if slices.ContainsFunc(b.posts, func(p post) bool { return p.AuthorID == id }) {
			web.Error(w, http.StatusConflict, "the author still has posts")
			return
		}
		b.authors = slices.Delete(b.authors, i, i+1)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /authors/{id}/posts", func(w http.ResponseWriter, r *http.Request) {
		limit, offset, ok := web.Paging(w, r)
		if !ok {
			return
		}
		b.mu.Lock()
		defer b.mu.Unlock()
		id := r.PathValue("id")
		if b.authorIndex(id) < 0 {
			web.Error(w, http.StatusNotFound, "author not found")
			return
		}
		var mine []post
		for _, p := range b.posts {
			if p.AuthorID == id {
				mine = append(mine, p)
			}
		}
		web.JSON(w, http.StatusOK, web.Page[post]{Items: web.Window(mine, limit, offset), Total: len(mine), Limit: limit, Offset: offset})
	})

	mux.HandleFunc("POST /posts", func(w http.ResponseWriter, r *http.Request) {
		var p post
		if !web.Decode(w, r, &p) {
			return
		}
		b.mu.Lock()
		defer b.mu.Unlock()
		if err := b.checkPost(p); err != nil {
			web.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		p.ID, p.CreatedAt = web.NewID(), time.Now().UTC()
		p.UpdatedAt = p.CreatedAt
		b.posts = append(b.posts, p)
		web.JSON(w, http.StatusCreated, p)
	})
	mux.HandleFunc("GET /posts", func(w http.ResponseWriter, r *http.Request) {
		limit, offset, ok := web.Paging(w, r)
		if !ok {
			return
		}
		b.mu.Lock()
		defer b.mu.Unlock()
		web.JSON(w, http.StatusOK, web.Page[post]{Items: web.Window(b.posts, limit, offset), Total: len(b.posts), Limit: limit, Offset: offset})
	})
	mux.HandleFunc("GET /posts/{id}", func(w http.ResponseWriter, r *http.Request) {
		b.mu.Lock()
		defer b.mu.Unlock()
		i := b.postIndex(r.PathValue("id"))
		if i < 0 {
			web.Error(w, http.StatusNotFound, "post not found")
			return
		}
		web.JSON(w, http.StatusOK, b.posts[i])
	})
	mux.HandleFunc("PUT /posts/{id}", func(w http.ResponseWriter, r *http.Request) {
		var in post
		if !web.Decode(w, r, &in) {
			return
		}
		b.mu.Lock()
		defer b.mu.Unlock()
		i := b.postIndex(r.PathValue("id"))
		if i < 0 {
			web.Error(w, http.StatusNotFound, "post not found")
			return
		}
		if err := b.checkPost(in); err != nil {
			web.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		p := &b.posts[i]
		p.AuthorID, p.Title, p.Body, p.Published, p.UpdatedAt = in.AuthorID, in.Title, in.Body, in.Published, time.Now().UTC()
		web.JSON(w, http.StatusOK, p)
	})
	mux.HandleFunc("DELETE /posts/{id}", func(w http.ResponseWriter, r *http.Request) {
		b.mu.Lock()
		defer b.mu.Unlock()
		i := b.postIndex(r.PathValue("id"))
		if i < 0 {
			web.Error(w, http.StatusNotFound, "post not found")
			return
		}
		b.posts = slices.Delete(b.posts, i, i+1)
		w.WriteHeader(http.StatusNoContent)
	})

	web.Serve(mux)
}
