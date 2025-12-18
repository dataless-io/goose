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

	b, err := broker.New(broker.Config{
		PersistencePath: persistencePath,
	})
	if err != nil {
		log.Fatalf("Failed to initialize broker: %v", err)
	}
	defer b.Close()

	handler := broker.NewHandler(b)

	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
