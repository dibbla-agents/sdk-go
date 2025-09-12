package main

import (
	"log"

	sdk "github.com/FatsharkStudiosAB/codex/workflows/workers/go/sdk"
	"github.com/FatsharkStudiosAB/codex/workflows/workers/go/worker/examples"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}

	// Create server with custom configuration
	server, err := sdk.New()
	if err != nil {
		log.Fatal("Failed to create server:", err)
	}

	// Register example functions using the SDK interface
	server.RegisterFunction(examples.InputFunction())
	server.RegisterFunction(examples.StoreChatHistoryFunction())

	// Start server (this will handle all initialization and block forever)
	log.Println("Starting server with SDK...")
	if err := server.Start(); err != nil {
		log.Fatal("Server failed:", err)
	}
}
