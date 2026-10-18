// Package security provides small, dependency-free primitives for integrity
// checking and human-readable diagnostic encodings. Binary and Morse output are
// encodings only; they are not encryption or authentication.
package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

var ErrEmptyKey = errors.New("integrity key must not be empty")

// Sign returns an HMAC-SHA-256 tag for data. The key must be supplied by the
// host from a secure secret store; do not hard-code or log it.
func Sign(key, data []byte) (string, error) {
	if len(key) == 0 {
		return "", ErrEmptyKey
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// Verify checks an HMAC-SHA-256 tag using constant-time comparison.
func Verify(key, data []byte, tag string) bool {
	if len(key) == 0 {
		return false
	}
	provided, err := hex.DecodeString(tag)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(data)
	return hmac.Equal(provided, mac.Sum(nil))
}

// Binary encodes each byte as eight 0/1 digits separated by spaces.
// This is for diagnostics, not confidentiality.
func Binary(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	var out strings.Builder
	out.Grow(len(data)*9 - 1)
	for i, b := range data {
		if i > 0 {
			out.WriteByte(' ')
		}
		for bit := 7; bit >= 0; bit-- {
			if b&(1<<bit) != 0 {
				out.WriteByte('1')
			} else {
				out.WriteByte('0')
			}
		}
	}
	return out.String()
}

var morse = map[rune]string{
	'A': ".-", 'B': "-...", 'C': "-.-.", 'D': "-..", 'E': ".",
	'F': "..-.", 'G': "--.", 'H': "....", 'I': "..", 'J': ".---",
	'K': "-.-", 'L': ".-..", 'M': "--", 'N': "-.", 'O': "---",
	'P': ".--.", 'Q': "--.-", 'R': ".-.", 'S': "...", 'T': "-",
	'U': "..-", 'V': "...-", 'W': ".--", 'X': "-..-", 'Y': "-.--",
	'Z': "--..", '0': "-----", '1': ".----", '2': "..---",
	'3': "...--", '4': "....-", '5': ".....", '6': "-....",
	'7': "--...", '8': "---..", '9': "----.",
}

// Morse encodes ASCII letters and digits. Spaces between words become " / ".
// Unsupported characters return an error rather than being silently changed.
func Morse(text string) (string, error) {
	text = strings.ToUpper(strings.TrimSpace(text))
	if text == "" {
		return "", nil
	}
	words := strings.Fields(text)
	encodedWords := make([]string, 0, len(words))
	for _, word := range words {
		symbols := make([]string, 0, len(word))
		for _, r := range word {
			code, ok := morse[r]
			if !ok {
				return "", errors.New("morse supports ASCII letters, digits, and spaces only")
			}
			symbols = append(symbols, code)
		}
		encodedWords = append(encodedWords, strings.Join(symbols, " "))
	}
	return strings.Join(encodedWords, " / "), nil
}
