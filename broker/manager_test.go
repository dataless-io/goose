package broker_test

import (
	"goose/broker"
	"os"
	"path/filepath"
	"testing"
)

func TestManager_CRUD(t *testing.T) {
	// Setup temp dir
	tmpDir, err := os.MkdirTemp("", "broker_manager_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	manager, err := broker.NewManager(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	// 1. Create Broker
	name := "test-broker"
	b, err := manager.CreateBroker(name)
	if err != nil {
		t.Fatalf("Failed to create broker: %v", err)
	}
	if b == nil {
		t.Fatal("Returned broker is nil")
	}

	// 2. Get Broker
	b2, exists := manager.GetBroker(name)
	if !exists {
		t.Fatal("Broker not found after creation")
	}
	if b != b2 {
		t.Fatal("GetBroker returned different instance")
	}

	// 3. Create Duplicate
	_, err = manager.CreateBroker(name)
	if err == nil {
		t.Fatal("Expected error creating duplicate broker")
	}

	// 4. Persistence verification
	logPath := filepath.Join(tmpDir, name+".log")
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Fatal("Persistence file not created")
	}

	// 5. Delete Broker
	if err := manager.DeleteBroker(name); err != nil {
		t.Fatalf("Failed to delete broker: %v", err)
	}

	// 6. Get Deleted
	_, exists = manager.GetBroker(name)
	if exists {
		t.Fatal("Broker still exists after deletion")
	}

	// 7. Re-create same broker name (should work)
	if _, err := manager.CreateBroker(name); err != nil {
		t.Fatalf("Failed to re-create deleted broker: %v", err)
	}
}
