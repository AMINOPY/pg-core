package security

import (
	"strings"
	"testing"
)

func TestSignVerifyAndTamperDetection(t *testing.T) {
	key := []byte("test-only-key")
	data := []byte("core integrity event")
	tag, err := Sign(key, data)
	if err != nil {
		t.Fatal(err)
	}
	if !Verify(key, data, tag) {
		t.Fatal("valid tag should verify")
	}
	if Verify(key, []byte("modified event"), tag) {
		t.Fatal("modified data must not verify")
	}
	if Verify([]byte("different key"), data, tag) {
		t.Fatal("different key must not verify")
	}
	if Verify(key, data, tag+"zz") {
		t.Fatal("malformed tag must not verify")
	}
}

func TestSignRejectsEmptyKey(t *testing.T) {
	if _, err := Sign(nil, []byte("data")); err != ErrEmptyKey {
		t.Fatalf("expected ErrEmptyKey, got %v", err)
	}
	if Verify(nil, []byte("data"), "00") {
		t.Fatal("empty key must never verify")
	}
}

func TestBinaryEncoding(t *testing.T) {
	if got, want := Binary([]byte("A")), "01000001"; got != want {
		t.Fatalf("Binary(A) = %q, want %q", got, want)
	}
	if got, want := Binary([]byte{0x00, 0xff}), "00000000 11111111"; got != want {
		t.Fatalf("Binary bytes = %q, want %q", got, want)
	}
	if got := Binary(nil); got != "" {
		t.Fatalf("Binary(nil) = %q, want empty", got)
	}
}

func TestMorseEncoding(t *testing.T) {
	got, err := Morse("SOS 2")
	if err != nil {
		t.Fatal(err)
	}
	if want := "... --- ... / ..---"; got != want {
		t.Fatalf("Morse(SOS 2) = %q, want %q", got, want)
	}
	if _, err := Morse("SAFE!"); err == nil {
		t.Fatal("unsupported punctuation should be rejected")
	}
	if got, err := Morse("   "); err != nil || got != "" {
		t.Fatalf("blank Morse = %q, %v; want empty and nil", got, err)
	}
	if strings.Contains(got, "!") {
		t.Fatal("unexpected punctuation in encoded output")
	}
}
