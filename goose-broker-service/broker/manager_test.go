package broker_test

import (
	"testing"

	"github.com/dataless-io/goose-broker-service/broker"
)

func TestManagerCRUDAndReloadFromDisk(t *testing.T) {
	dir := t.TempDir()

	m, err := broker.NewManager(dir)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	b, err := m.CreateBroker("orders")
	if err != nil {
		t.Fatalf("create broker: %v", err)
	}

	if err := b.Send("events", map[string]string{"id": "o-1"}); err != nil {
		t.Fatalf("send: %v", err)
	}

	if err := m.Close(); err != nil {
		t.Fatalf("close manager: %v", err)
	}

	reloaded, err := broker.NewManager(dir)
	if err != nil {
		t.Fatalf("reload manager: %v", err)
	}
	defer reloaded.Close()

	if _, ok := reloaded.GetBroker("orders"); !ok {
		t.Fatalf("expected broker loaded from disk")
	}
}

func TestManagerRejectsInvalidBrokerNames(t *testing.T) {
	m, err := broker.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	invalid := []string{"", "../x", "x/y", "bad name", "x*"}
	for _, name := range invalid {
		if _, err := m.CreateBroker(name); err == nil {
			t.Fatalf("expected invalid name %q to fail", name)
		}
	}
}

func TestManagerBrokerSettingsPersist(t *testing.T) {
	dir := t.TempDir()
	m, err := broker.NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := m.CreateBroker("events-prod"); err != nil {
		t.Fatal(err)
	}

	disabled := true
	display := "Events Production"
	desc := "primary production broker"
	if _, err := m.UpdateBrokerSettings("events-prod", broker.BrokerSettingsUpdate{
		DisplayName: &display,
		Description: &desc,
		Disabled:    &disabled,
	}); err != nil {
		t.Fatal(err)
	}

	if err := m.Close(); err != nil {
		t.Fatal(err)
	}

	reloaded, err := broker.NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reloaded.Close()

	settings, ok := reloaded.GetBrokerSettings("events-prod")
	if !ok {
		t.Fatalf("expected settings for events-prod")
	}
	if settings.DisplayName != display || settings.Description != desc || !settings.Disabled {
		t.Fatalf("settings mismatch after reload: %+v", settings)
	}
}
