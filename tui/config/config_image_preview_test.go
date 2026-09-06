// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2026 Tulir Asokan, buihongduc132
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfigFile(t *testing.T, cfg *Config, content string) {
	t.Helper()
	path := filepath.Join(cfg.Dir, "terminal.yaml")
	if err := os.MkdirAll(cfg.Dir, 0700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write terminal.yaml: %v", err)
	}
}

func TestConfig_ImagePreviewDefaults(t *testing.T) {
	cfg := newTestConfig(t)
	cfg.Load()

	if cfg.Preferences.ImagePreviewProtocol != "auto" {
		t.Errorf("expected default ImagePreviewProtocol to be 'auto', got %q", cfg.Preferences.ImagePreviewProtocol)
	}
	if cfg.Preferences.ImagePreviewMaxWidth != 66 {
		t.Errorf("expected default ImagePreviewMaxWidth to be 66, got %d", cfg.Preferences.ImagePreviewMaxWidth)
	}
	if cfg.Preferences.ImagePreviewMaxHeight != 16 {
		t.Errorf("expected default ImagePreviewMaxHeight to be 16, got %d", cfg.Preferences.ImagePreviewMaxHeight)
	}
}

func TestConfig_ImagePreviewUnderPreferences(t *testing.T) {
	cfg := newTestConfig(t)
	writeConfigFile(t, cfg, `
preferences:
  image_preview_protocol: halfblocks
  image_preview_max_width: 80
  image_preview_max_height: 24
`)
	cfg.Load()

	if cfg.Preferences.ImagePreviewProtocol != "halfblocks" {
		t.Errorf("expected ImagePreviewProtocol 'halfblocks', got %q", cfg.Preferences.ImagePreviewProtocol)
	}
	if cfg.Preferences.ImagePreviewMaxWidth != 80 {
		t.Errorf("expected ImagePreviewMaxWidth 80, got %d", cfg.Preferences.ImagePreviewMaxWidth)
	}
	if cfg.Preferences.ImagePreviewMaxHeight != 24 {
		t.Errorf("expected ImagePreviewMaxHeight 24, got %d", cfg.Preferences.ImagePreviewMaxHeight)
	}
}

func TestConfig_ImagePreviewBlock(t *testing.T) {
	cfg := newTestConfig(t)
	writeConfigFile(t, cfg, `
image_preview:
  protocol: iterm2
  max_width: 72
  max_height: 20
`)
	cfg.Load()

	if cfg.Preferences.ImagePreviewProtocol != "iterm2" {
		t.Errorf("expected ImagePreviewProtocol 'iterm2', got %q", cfg.Preferences.ImagePreviewProtocol)
	}
	if cfg.Preferences.ImagePreviewMaxWidth != 72 {
		t.Errorf("expected ImagePreviewMaxWidth 72, got %d", cfg.Preferences.ImagePreviewMaxWidth)
	}
	if cfg.Preferences.ImagePreviewMaxHeight != 20 {
		t.Errorf("expected ImagePreviewMaxHeight 20, got %d", cfg.Preferences.ImagePreviewMaxHeight)
	}
}

func TestConfig_ImagePreviewTopLevel(t *testing.T) {
	cfg := newTestConfig(t)
	writeConfigFile(t, cfg, `
image_preview_protocol: disabled
image_preview_max_width: 50
image_preview_max_height: 12
`)
	cfg.Load()

	if cfg.Preferences.ImagePreviewProtocol != "disabled" {
		t.Errorf("expected ImagePreviewProtocol 'disabled', got %q", cfg.Preferences.ImagePreviewProtocol)
	}
	if cfg.Preferences.ImagePreviewMaxWidth != 50 {
		t.Errorf("expected ImagePreviewMaxWidth 50, got %d", cfg.Preferences.ImagePreviewMaxWidth)
	}
	if cfg.Preferences.ImagePreviewMaxHeight != 12 {
		t.Errorf("expected ImagePreviewMaxHeight 12, got %d", cfg.Preferences.ImagePreviewMaxHeight)
	}
}

func TestConfig_ImagePreviewPartialOverride(t *testing.T) {
	cfg := newTestConfig(t)
	writeConfigFile(t, cfg, `
preferences:
  image_preview_protocol: halfblocks
`)
	cfg.Load()

	if cfg.Preferences.ImagePreviewProtocol != "halfblocks" {
		t.Errorf("expected ImagePreviewProtocol 'halfblocks', got %q", cfg.Preferences.ImagePreviewProtocol)
	}
	// Defaults should remain for unconfigured fields
	if cfg.Preferences.ImagePreviewMaxWidth != 66 {
		t.Errorf("expected default ImagePreviewMaxWidth 66, got %d", cfg.Preferences.ImagePreviewMaxWidth)
	}
	if cfg.Preferences.ImagePreviewMaxHeight != 16 {
		t.Errorf("expected default ImagePreviewMaxHeight 16, got %d", cfg.Preferences.ImagePreviewMaxHeight)
	}
}
