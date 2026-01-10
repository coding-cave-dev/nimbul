package httpserver

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v2"
)

type HealthCheckResponse struct {
	Body struct {
		Message string `json:"message"`
	}
}

func NewRouter() *fiber.App {
	app := fiber.New()

	api := humafiber.New(app, huma.DefaultConfig("Nimbul API", "1.0.0"))

	huma.Get(api, "/health", func(ctx context.Context, input *struct{}) (*HealthCheckResponse, error) {
		resp := &HealthCheckResponse{}
		resp.Body.Message = "Nimbul API is up and running"
		return resp, nil
	})

	return app
}
