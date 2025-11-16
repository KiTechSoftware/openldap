package main

import (
	"context"
	"log"

	"github.com/kitechsoftware/ldappy/internal/api/app"
	"github.com/kitechsoftware/ldappy/internal/api/handlers"
	"github.com/kitechsoftware/ldappy/internal/common/core"
)

func main() {
	ctx := context.Background()
	cfg, err := core.Load(ctx, "")
	if err != nil {
		log.Fatalf("❌ Failed to load config: %v", err)
	}

	app := app.NewApp(ctx, cfg)

	// CRUD operations
	app.AddRoute("/ping", handlers.PingHandler)
	app.AddRoute("/add", handlers.AddHandler)
	app.AddRoute("/modify", handlers.ModifyHandler)
	app.AddRoute("/delete", handlers.DeleteHandler)
	app.AddRoute("/search", handlers.SearchHandler)

	// Password management
	app.AddRoute("/password/hash", handlers.PasswordHashHandler)
	app.AddRoute("/password/reset", handlers.PasswordResetHandler)

	// Start the server
	if err := app.Start(); err != nil {
		log.Fatalf("❌ Failed to start: %v", err)
	}
}
