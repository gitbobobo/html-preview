package item

import (
	"testing"
	"time"
)

func TestParseExpires_EmptyDefaultsTo30Days(t *testing.T) {
	got, err := ParseExpires("", "")
	if err != nil {
		t.Fatalf("ParseExpires: %v", err)
	}
	if got == nil {
		t.Fatal("expected 30d default, got never")
	}
	want := 30 * 24 * time.Hour
	delta := got.Sub(time.Now().UTC())
	if delta < want-2*time.Second || delta > want+2*time.Second {
		t.Fatalf("default expiry delta = %v, want ~%v", delta, want)
	}
}

func TestParseExpires_Never(t *testing.T) {
	got, err := ParseExpires("never", "")
	if err != nil {
		t.Fatalf("ParseExpires: %v", err)
	}
	if got != nil {
		t.Fatalf("expected never, got %v", got)
	}
}

func TestParseExpires_ExplicitExpiresAtWins(t *testing.T) {
	at := "2030-01-15T00:00:00Z"
	got, err := ParseExpires("", at)
	if err != nil {
		t.Fatalf("ParseExpires: %v", err)
	}
	if got == nil || got.Format(time.RFC3339) != at {
		t.Fatalf("got %v, want %s", got, at)
	}
}
