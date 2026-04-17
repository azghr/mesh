// Package json provides fast JSON encoding and decoding.
//
// This package wraps high-performance JSON libraries with a compatible API.
// It uses goccy/go-json for best performance without reflection.
//
// # Usage
//
// Replace standard library usage:
//
//	import "github.com/azghr/mesh/json"
//
//	// Instead of json.Marshal
//	data, err := json.Marshal(v)
//
//	// Instead of json.Unmarshal
//	err := json.Unmarshal(data, &v)
//
//	// Encoder for streaming
//	enc := json.NewEncoder(w)
//	err := enc.Encode(v)
//
//	// Decoder for streaming
//	dec := json.NewDecoder(r)
//	err := dec.Decode(&v)
//
// # Performance
//
// goccy/go-json provides 2-10x faster encoding than the standard library
// without using reflection. It also produces deterministic output.
//
// # Compatibility
//
// API is compatible with encoding/json. Replace imports to use.
package json

import (
	"io"

	goccy "github.com/goccy/go-json"
)

// Marshal encodes a value to JSON.
func Marshal(v interface{}) ([]byte, error) {
	return goccy.Marshal(v)
}

// MarshalIndent encodes a value to JSON with indentation.
func MarshalIndent(v interface{}, prefix, indent string) ([]byte, error) {
	return goccy.MarshalIndent(v, prefix, indent)
}

// Unmarshal decodes JSON into a value.
func Unmarshal(data []byte, v interface{}) error {
	return goccy.Unmarshal(data, v)
}

// NewEncoder returns a new encoder writing to w.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{enc: goccy.NewEncoder(w)}
}

// NewDecoder returns a new decoder reading from r.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{dec: goccy.NewDecoder(r)}
}

// Encoder wraps goccy/go-json encoder.
type Encoder struct {
	enc *goccy.Encoder
}

// Encode writes the JSON encoding of v.
func (e *Encoder) Encode(v interface{}) error {
	return e.enc.Encode(v)
}

// SetIndent sets the indentation.
func (e *Encoder) SetIndent(prefix, indent string) {
	e.enc.SetIndent(prefix, indent)
}

// SetEscapeHTML sets whether to escape HTML.
func (e *Encoder) SetEscapeHTML(escape bool) {
	e.enc.SetEscapeHTML(escape)
}

// Decoder wraps goccy/go-json decoder.
type Decoder struct {
	dec *goccy.Decoder
}

// Decode reads the next JSON-encoded value into v.
func (d *Decoder) Decode(v interface{}) error {
	return d.dec.Decode(v)
}

// UseNumber causes Decode to unmarshal a number into an interface{} as a Number.
func (d *Decoder) UseNumber() {
	d.dec.UseNumber()
}

// Valid reports whether data is a valid JSON encoding.
func Valid(data []byte) bool {
	return goccy.Valid(data)
}
