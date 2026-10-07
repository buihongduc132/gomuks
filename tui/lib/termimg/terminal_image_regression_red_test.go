// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2026 Tulir Asokan, buihongduc132
//
// RED regression tests for user-visible terminal image failures discovered after PR #4.

package termimg

import (
	"bytes"
	"encoding/base64"
	"image"
	"strings"
	"testing"
)

func TestRED_WezTermAutoSelectsITerm2InsteadOfKittyUnicodePlaceholder(t *testing.T) {
	tests := []struct {
		name string
		tmux bool
	}{
		{name: "direct"},
		{name: "inside_tmux_with_passthrough", tmux: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clearEnvs(t)
			t.Setenv("TERM_PROGRAM", "WezTerm")
			if tc.tmux {
				t.Setenv("TMUX", "/tmp/tmux-1000/default,1234,0")
			}

			oldChecker := tmuxPassthroughChecker
			defer func() { tmuxPassthroughChecker = oldChecker }()
			tmuxPassthroughChecker = func() bool { return tc.tmux }

			cap := DetectTerminal()
			if cap.Protocol != ProtocolITerm2 {
				t.Fatalf("WezTerm must avoid Kitty Unicode-placeholder mode; got protocol %q, want %q", cap.Protocol, ProtocolITerm2)
			}
		})
	}
}

func TestRED_TmuxKittyPassthroughEscapesInnerStringTerminator(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	seq := FormatKittyTransmit(img, 7, true)

	wantPrefix := []byte("\x1bPtmux;\x1b\x1b_Gq=2,")
	if !bytes.HasPrefix(seq, wantPrefix) {
		t.Fatalf("tmux Kitty sequence missing escaped inner APC prefix: %q", seq)
	}

	// tmux passthrough requires ESC bytes in the inner APC terminator to be doubled,
	// followed by the outer DCS terminator: ESC ESC \\ ESC \\.
	wantSuffix := []byte("\x1b\x1b\\\x1b\\")
	if !bytes.HasSuffix(seq, wantSuffix) {
		t.Fatalf("tmux Kitty sequence must escape the inner ST; got suffix %q, want %q", tailBytes(seq, len(wantSuffix)), wantSuffix)
	}
}

func TestRED_KittyTransmitPacksOnlyVisibleRGBARows(t *testing.T) {
	// 2 visible RGBA pixels per row = 8 bytes, but Stride=12 adds 4 padding bytes.
	// Sending rgba.Pix directly leaks padding into the Kitty f=32 payload and corrupts rows.
	pix := []byte{
		1, 2, 3, 4, 5, 6, 7, 8, 201, 202, 203, 204,
		9, 10, 11, 12, 13, 14, 15, 16, 211, 212, 213, 214,
	}
	img := &image.RGBA{Pix: pix, Stride: 12, Rect: image.Rect(0, 0, 2, 2)}

	seq := FormatKittyTransmit(img, 9, false)
	payload := kittySingleChunkPayload(t, seq)
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("decode Kitty payload: %v", err)
	}

	want := []byte{
		1, 2, 3, 4, 5, 6, 7, 8,
		9, 10, 11, 12, 13, 14, 15, 16,
	}
	if !bytes.Equal(decoded, want) {
		t.Fatalf("Kitty RGBA payload contains stride padding: got %v, want tightly packed %v", decoded, want)
	}
}

func TestRED_KittyPlaceholderRejectsUnencodableCoordinates(t *testing.T) {
	max := len(kittyDiacritics)
	if got := CreateKittyPlaceholderBuffer(max+1, 1, 1); got != nil {
		t.Fatalf("columns beyond Kitty diacritic coordinate range must be rejected, got %d rows", len(got))
	}
	if got := CreateKittyPlaceholderBuffer(1, max+1, 1); got != nil {
		t.Fatalf("rows beyond Kitty diacritic coordinate range must be rejected, got %d rows", len(got))
	}
}

func kittySingleChunkPayload(t *testing.T, seq []byte) string {
	t.Helper()
	s := string(seq)
	semi := strings.IndexByte(s, ";"[0])
	if semi < 0 {
		t.Fatalf("Kitty sequence has no payload separator: %q", s)
	}
	end := strings.LastIndex(s, "\x1b\\")
	if end <= semi {
		t.Fatalf("Kitty sequence has no final ST: %q", s)
	}
	return s[semi+1 : end]
}

func tailBytes(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return b[len(b)-n:]
}
