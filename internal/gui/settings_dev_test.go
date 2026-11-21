//go:build !production

package gui

import (
	"testing"

	"github.com/ken19960728ken/preflop-matrix-app/internal/config"
)

func TestSettingsPathOverride(t *testing.T) {
	t.Setenv("PMA_SETTINGS_PATH", "/x/settings.json")
	if p, err := settingsPath(); err != nil || p != "/x/settings.json" {
		t.Errorf("settingsPath = %q, %v", p, err)
	}
	t.Setenv("PMA_SETTINGS_PATH", "")
	want, err := config.SettingsPath()
	if err != nil {
		t.Fatal(err)
	}
	if p, _ := settingsPath(); p != want {
		t.Errorf("settingsPath = %q, want %q", p, want)
	}
}
