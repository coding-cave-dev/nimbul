package main

import (
	"github.com/nimbul/internal/httpserver"
)

func main() {
	router := httpserver.NewRouter()
	router.Listen(":8080")
}
