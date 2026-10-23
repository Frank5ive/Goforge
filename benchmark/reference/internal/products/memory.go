package products

import (
	"slices"
	"sync"

	"github.com/Frank5ive/goforge/benchmark/reference/internal/web"
)

// Memory is a Store in memory.
type Memory struct {
	mu    sync.Mutex
	items []Product
}

func (m *Memory) List(limit, offset int) ([]Product, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(web.Window(m.items, limit, offset)), len(m.items), nil
}

func (m *Memory) Get(id string) (Product, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.items {
		if p.ID == id {
			return p, nil
		}
	}
	return Product{}, web.ErrNotFound
}

func (m *Memory) Create(p Product) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items = append(m.items, p)
	return nil
}

func (m *Memory) Update(p Product) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.items {
		if m.items[i].ID == p.ID {
			m.items[i] = p
			return nil
		}
	}
	return web.ErrNotFound
}

func (m *Memory) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.items {
		if m.items[i].ID == id {
			m.items = slices.Delete(m.items, i, i+1)
			return nil
		}
	}
	return web.ErrNotFound
}
