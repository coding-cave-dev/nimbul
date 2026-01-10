package main

import (
	"os"

	"github.com/coding-cave-dev/nimbul/internal/httpserver"
)

func main() {
	router := httpserver.NewRouter()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	if err := router.Listen(":" + port); err != nil {
		panic(err)
	}
}
