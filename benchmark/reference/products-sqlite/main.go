// Command products-sqlite is the reference solution for the products-sqlite task.
package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	_ "modernc.org/sqlite"

	"github.com/Frank5ive/goforge/benchmark/reference/internal/products"
	"github.com/Frank5ive/goforge/benchmark/reference/internal/web"
)

func main() {
	db, err := sql.Open("sqlite", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	store := &products.SQL{DB: db}
	if err := store.Migrate(); err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	products.Register(mux, store)
	web.Serve(mux)
}
