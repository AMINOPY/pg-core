package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"testing"
)

// FuzzLoadJSON exercises the strict JSON decoding and schema validation
// boundary with arbitrary input. The production loader additionally handles
// filesystem errors; this target focuses on untrusted file contents.
func FuzzLoadJSON(f *testing.F) {
	seeds := [][]byte{
		[]byte(`{}`),
		[]byte(`{"schema_version":1,"profile":"test","active_engine":"sing-box"}`),
		[]byte(`{"schema_version":999,"profile":"test","active_engine":"sing-box"}`),
		[]byte(`{"profile":"test","active_engine":"unknown","extra":true}`),
		[]byte(`{} {}`),
		[]byte(`null`),
		[]byte(`[`),
		{},
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input []byte) {
		dec := json.NewDecoder(bytes.NewReader(input))
		dec.DisallowUnknownFields()

		var cfg Config
		if err := dec.Decode(&cfg); err != nil {
			return
		}

		var extra any
		err := dec.Decode(&extra)
		if !errors.Is(err, io.EOF) {
			return
		}

		normalized, err := cfg.Normalize()
		if err != nil {
			return
		}
		_ = normalized.Validate()
	})
}
