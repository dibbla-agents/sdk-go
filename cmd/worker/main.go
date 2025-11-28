package main

import (
	"log"

	"github.com/dibbla-agents/sdk-go"
	"github.com/dibbla-agents/sdk-go/cmd/worker/examples"

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

	// OAuth example functions
	server.RegisterFunction(examples.GetGoogleTokenFunction())
	server.RegisterFunction(examples.CheckConnectedProvidersFunction())

	// Google Sheets examples
	server.RegisterFunction(examples.ReadGoogleSheetsFunction())
	server.RegisterFunction(examples.UpdateGoogleSheetsFunction())

	// Start server (this will handle all initialization and block forever)
	log.Println("Starting server with SDK...")
	if err := server.Start(); err != nil {
		log.Fatal("Server failed:", err)
	}
}
