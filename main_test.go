package main

import "testing"

func TestDisableUpdatesRegardlessOfEmbeddedOrBuildSource(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
		key    string
	}{
		{name: "embedded upstream source", source: "https://example.invalid/upstream/update.json", key: "upstream-key"},
		{name: "build time override", source: "file:///local-test-updates", key: "override-key"},
		{name: "partially configured", source: "file:///local-test-updates"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := appConfig{ID: "original.app.id", Version: "0.1.0", UpdateSource: tc.source, UpdatePublicKey: tc.key}
			disableUpdates(&cfg)
			if cfg.UpdateSource != "" || cfg.UpdatePublicKey != "" {
				t.Fatalf("updater must be disabled: source=%q key=%q", cfg.UpdateSource, cfg.UpdatePublicKey)
			}
			if cfg.ID != "original.app.id" || cfg.Version != "0.1.0" {
				t.Fatal("updater disablement unexpectedly modified app identity or version")
			}
		})
	}
}
