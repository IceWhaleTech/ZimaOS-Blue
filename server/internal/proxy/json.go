package proxy

import (
	stdjson "encoding/json"
	"io"
)

type rawJSONMessage = stdjson.RawMessage

// jsonAPI wraps encoding/json as a package-level variable so all files
// in the proxy package can call json.Marshal, json.Unmarshal, etc.
type jsonAPI struct{}

func (jsonAPI) Marshal(v interface{}) ([]byte, error)      { return stdjson.Marshal(v) }
func (jsonAPI) Unmarshal(data []byte, v interface{}) error { return stdjson.Unmarshal(data, v) }
func (jsonAPI) MarshalIndent(v interface{}, prefix, indent string) ([]byte, error) {
	return stdjson.MarshalIndent(v, prefix, indent)
}
func (jsonAPI) NewEncoder(w io.Writer) *stdjson.Encoder { return stdjson.NewEncoder(w) }
func (jsonAPI) NewDecoder(r io.Reader) *stdjson.Decoder { return stdjson.NewDecoder(r) }
func (jsonAPI) Valid(data []byte) bool                  { return stdjson.Valid(data) }

// json is a package-local wrapper around encoding/json.
// All files in the proxy package use this variable for JSON operations.
var json jsonAPI
