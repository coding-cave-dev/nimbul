/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package main

import (
	cli "github.com/coding-cave-dev/nimbul/internal/cli"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		panic(err)
	}
	cli.Execute()
}
