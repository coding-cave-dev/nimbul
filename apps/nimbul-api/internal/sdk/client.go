package sdk

import (
	"os"
)

func DefaultNimbulClient() *Client {
	baseURL := os.Getenv("NIMBUL_API_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	client, err := NewClient(baseURL)
	if err != nil {
		panic(err)
	}
	return client
}
