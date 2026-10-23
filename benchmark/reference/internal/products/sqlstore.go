package products

import (
	"database/sql"
	"errors"
	"time"

	"github.com/Frank5ive/goforge/benchmark/reference/internal/web"
)

// SQL is a Store in a SQLite database.
type SQL struct {
	DB *sql.DB
}

// Migrate creates the products table if it doesn't exist.
func (s *SQL) Migrate() error {
	_, err := s.DB.Exec(`CREATE TABLE IF NOT EXISTS products (
		seq        INTEGER PRIMARY KEY AUTOINCREMENT,
		id         TEXT NOT NULL UNIQUE,
		name       TEXT NOT NULL,
		price      REAL NOT NULL,
		in_stock   INTEGER NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`)
	return err
}

func (s *SQL) List(limit, offset int) ([]Product, int, error) {
	var total int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM products`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.DB.Query(`SELECT id, name, price, in_stock, created_at, updated_at FROM products ORDER BY seq LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []Product{}
	for rows.Next() {
		p, err := scan(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, p)
	}
	return items, total, rows.Err()
}

func (s *SQL) Get(id string) (Product, error) {
	p, err := scan(s.DB.QueryRow(`SELECT id, name, price, in_stock, created_at, updated_at FROM products WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Product{}, web.ErrNotFound
	}
	return p, err
}

func (s *SQL) Create(p Product) error {
	_, err := s.DB.Exec(`INSERT INTO products (id, name, price, in_stock, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Price, p.InStock, p.CreatedAt.Format(time.RFC3339Nano), p.UpdatedAt.Format(time.RFC3339Nano))
	return err
}

func (s *SQL) Update(p Product) error {
	res, err := s.DB.Exec(`UPDATE products SET name = ?, price = ?, in_stock = ?, updated_at = ? WHERE id = ?`,
		p.Name, p.Price, p.InStock, p.UpdatedAt.Format(time.RFC3339Nano), p.ID)
	return affected(res, err)
}

func (s *SQL) Delete(id string) error {
	res, err := s.DB.Exec(`DELETE FROM products WHERE id = ?`, id)
	return affected(res, err)
}

func affected(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return web.ErrNotFound
	}
	return nil
}

type scanner interface{ Scan(...any) error }

func scan(row scanner) (Product, error) {
	var p Product
	var created, updated string
	if err := row.Scan(&p.ID, &p.Name, &p.Price, &p.InStock, &created, &updated); err != nil {
		return Product{}, err
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	p.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return p, nil
}
