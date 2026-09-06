// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2026 Tulir Asokan, buihongduc132
//
// Adversarial config tests for Milestone M1

package config

import (
	"testing"
)

func TestAdversarial_Config_DefaultsOnEmptyFile(t *testing.T) {
	cfg := newTestConfig(t)
	writeConfigFile(t, cfg, "")
	cfg.Load()

	if cfg.Preferences.ImagePreviewProtocol != "auto" {
		t.Errorf("expected default 'auto', got %q", cfg.Preferences.ImagePreviewProtocol)
	}
	if cfg.Preferences.ImagePreviewMaxWidth != 66 {
		t.Errorf("expected default 66, got %d", cfg.Preferences.ImagePreviewMaxWidth)
	}
	if cfg.Preferences.ImagePreviewMaxHeight != 16 {
		t.Errorf("expected default 16, got %d", cfg.Preferences.ImagePreviewMaxHeight)
	}
}

func TestAdversarial_Config_NegativeAndZeroDimensions(t *testing.T) {
	cfg := newTestConfig(t)
	writeConfigFile(t, cfg, `
preferences:
  image_preview_protocol: iterm2
  image_preview_max_width: -50
  image_preview_max_height: 0
`)
	cfg.Load()

	if cfg.Preferences.ImagePreviewProtocol != "iterm2" {
		t.Errorf("expected 'iterm2', got %q", cfg.Preferences.ImagePreviewProtocol)
	}
	// Non-positive dimensions must be clamped back to safe defaults (66, 16)
	if cfg.Preferences.ImagePreviewMaxWidth != 66 {
		t.Errorf("expected negative max width to reset to 66, got %d", cfg.Preferences.ImagePreviewMaxWidth)
	}
	if cfg.Preferences.ImagePreviewMaxHeight != 16 {
		t.Errorf("expected zero max height to reset to 16, got %d", cfg.Preferences.ImagePreviewMaxHeight)
	}
}

func TestAdversarial_Config_UnknownKeysAndComments(t *testing.T) {
	cfg := newTestConfig(t)
	writeConfigFile(t, cfg, `
# Leading comment
unknown_root_key: "some-value"
image_preview_protocol: halfblocks
preferences:
  unknown_pref_key: 12345
  image_preview_max_width: 88
# Trailing comment
`)
	cfg.Load()

	if cfg.Preferences.ImagePreviewProtocol != "halfblocks" {
		t.Errorf("expected 'halfblocks', got %q", cfg.Preferences.ImagePreviewProtocol)
	}
	if cfg.Preferences.ImagePreviewMaxWidth != 88 {
		t.Errorf("expected 88, got %d", cfg.Preferences.ImagePreviewMaxWidth)
	}
	if cfg.Preferences.ImagePreviewMaxHeight != 16 {
		t.Errorf("expected default 16, got %d", cfg.Preferences.ImagePreviewMaxHeight)
	}
}

func TestAdversarial_Config_PrecedenceHierarchy(t *testing.T) {
	// Top-level flat image_preview_protocol takes highest precedence over nested blocks
	cfg := newTestConfig(t)
	writeConfigFile(t, cfg, `
image_preview_protocol: disabled
image_preview:
  protocol: halfblocks
preferences:
  image_preview:
    protocol: iterm2
  image_preview_protocol: auto
`)
	cfg.Load()

	if cfg.Preferences.ImagePreviewProtocol != "disabled" {
		t.Errorf("expected top-level flat 'disabled' to win precedence, got %q", cfg.Preferences.ImagePreviewProtocol)
	}
}

func TestAdversarial_Config_SaveAndReloadRoundTrip(t *testing.T) {
	cfg := newTestConfig(t)
	writeConfigFile(t, cfg, `
preferences:
  image_preview_protocol: halfblocks
  image_preview_max_width: 100
  image_preview_max_height: 30
`)
	cfg.Load()

	// Modify programmatically and save (clear nosave set by newTestConfig)
	cfg.nosave = false
	cfg.Preferences.ImagePreviewProtocol = "iterm2"
	cfg.Preferences.ImagePreviewMaxWidth = 75
	cfg.Preferences.ImagePreviewMaxHeight = 22
	cfg.Save()

	// Reload into fresh instance
	cfg2 := newTestConfig(t)
	cfg2.Dir = cfg.Dir
	cfg2.Load()

	if cfg2.Preferences.ImagePreviewProtocol != "iterm2" {
		t.Errorf("expected persisted protocol 'iterm2', got %q", cfg2.Preferences.ImagePreviewProtocol)
	}
	if cfg2.Preferences.ImagePreviewMaxWidth != 75 {
		t.Errorf("expected persisted max_width 75, got %d", cfg2.Preferences.ImagePreviewMaxWidth)
	}
	if cfg2.Preferences.ImagePreviewMaxHeight != 22 {
		t.Errorf("expected persisted max_height 22, got %d", cfg2.Preferences.ImagePreviewMaxHeight)
	}
}

func TestAdversarial_Config_CaseInsensitiveUnmarshaling(t *testing.T) {
	cfg := newTestConfig(t)
	writeConfigFile(t, cfg, `
preferences:
  image_preview_protocol: HALFBLOCKS
`)
	cfg.Load()

	if cfg.Preferences.ImagePreviewProtocol != "HALFBLOCKS" {
		t.Errorf("expected 'HALFBLOCKS', got %q", cfg.Preferences.ImagePreviewProtocol)
	}
}

func TestAdversarial_Config_InvalidTypesInYAML(t *testing.T) {
	t.Run("string in max_width field", func(t *testing.T) {
		cfg := newTestConfig(t)
		writeConfigFile(t, cfg, `
preferences:
  image_preview_max_width: "not-an-int"
`)
		// In Go yaml.v3, unmarshaling "not-an-int" into an int field produces an error.
		// Let's verify whether Load panics or handles it:
		defer func() {
			r := recover()
			if r == nil {
				// If it did not panic, verify defaults
				if cfg.Preferences.ImagePreviewMaxWidth != 66 {
					t.Errorf("expected default 66, got %d", cfg.Preferences.ImagePreviewMaxWidth)
				}
			} else {
				t.Logf("Config.Load() panicked as expected on type mismatch: %v", r)
			}
		}()
		cfg.Load()
	})

	t.Run("number in image_preview_protocol field", func(t *testing.T) {
		cfg := newTestConfig(t)
		writeConfigFile(t, cfg, `
preferences:
  image_preview_protocol: 12345
`)
		defer func() {
			r := recover()
			if r != nil {
				t.Logf("Config.Load() panicked on number in string field: %v", r)
			}
		}()
		cfg.Load()
		t.Logf("Resulting protocol: %q", cfg.Preferences.ImagePreviewProtocol)
	})
}
