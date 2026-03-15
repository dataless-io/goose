package main

import (
	"log"
	"net/http"
	"os"

	"goose/broker"
)

func main() {
	log.Println("Starting Goose Broker...")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	persistencePath := os.Getenv("PERSISTENCE_PATH")
	if persistencePath == "" {
		persistencePath = "events.log"
	}

	log.Printf("Persistence Path: %s", persistencePath)
	log.Printf("Listening on port: %s", port)

	// 1. Initialize Manager
	manager, err := broker.NewManager(persistencePath)
	if err != nil {
		log.Fatalf("Failed to initialize manager: %v", err)
	}
	defer manager.Close()

	// 2. Create Default Broker (optional, but good for quickstart)
	if _, err := manager.CreateBroker("default"); err != nil {
		log.Printf("Default broker might already exist or failed: %v", err)
	} else {
		log.Println("Created 'default' broker")
	}

	handler := broker.NewHandler(manager)

	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
