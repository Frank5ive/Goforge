// Command products is the reference solution for the products task.
package main

import (
	"net/http"

	"github.com/Frank5ive/goforge/benchmark/reference/internal/products"
	"github.com/Frank5ive/goforge/benchmark/reference/internal/web"
)

func main() {
	mux := http.NewServeMux()
	products.Register(mux, &products.Memory{})
	web.Serve(mux)
}
