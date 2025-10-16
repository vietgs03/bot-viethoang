package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"bot-viethoang/internal/di"
)

func main() {
	application, err := di.InitializeApp()
	if err != nil {
		log.Fatalf("failed to initialize application: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := application.Run(ctx); err != nil {
		log.Fatalf("application runtime error: %v", err)
	}
}
