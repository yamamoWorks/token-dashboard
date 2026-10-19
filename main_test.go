package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	"token-monitor-turzx/internal/appstate"
	"token-monitor-turzx/internal/updates"
)

// Use the real embedded metadata, including the restored upstream release URL.
// CI also runs this test with nonempty linker overrides to protect that build path.
func TestCompactUpdaterWithEmbeddedMetadataAndLinkerOverrides(t *testing.T) {
	var cfg appConfig
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		t.Fatal(err)
	}
	if buildUpdateSource != "" {
		cfg.UpdateSource = buildUpdateSource
	}
	if buildUpdatePublicKey != "" {
		cfg.UpdatePublicKey = buildUpdatePublicKey
	}
	disableUpdates(&cfg)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	u := updates.New(updates.Config{
		AppID: cfg.ID, Version: cfg.Version, Source: cfg.UpdateSource,
		PublicKey: cfg.UpdatePublicKey, Enabled: false, CacheDir: t.TempDir(),
	}, &appstate.State{}, logger, func(string, any) {}, func(string) error {
		t.Fatal("Compact must never launch an installer")
		return nil
	}, func() { t.Fatal("Compact must never approve an updater restart") })
	if u.GetStatus().Configured {
		t.Fatal("restored metadata or linker values configured the Compact updater")
	}
	if _, err := u.Check(context.Background()); err == nil {
		t.Fatal("Compact updater accepted a manual update check")
	}
}

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
