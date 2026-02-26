package proxy

import (
	"io"

	gojson "github.com/goccy/go-json"
)

// jsonAPI wraps goccy/go-json as a package-level variable so all files
// in the proxy package can call json.Marshal, json.Unmarshal, etc.
type jsonAPI struct{}

func (jsonAPI) Marshal(v interface{}) ([]byte, error)          { return gojson.Marshal(v) }
func (jsonAPI) Unmarshal(data []byte, v interface{}) error     { return gojson.Unmarshal(data, v) }
func (jsonAPI) MarshalIndent(v interface{}, prefix, indent string) ([]byte, error) {
	return gojson.MarshalIndent(v, prefix, indent)
}
func (jsonAPI) NewEncoder(w io.Writer) *gojson.Encoder         { return gojson.NewEncoder(w) }
func (jsonAPI) NewDecoder(r io.Reader) *gojson.Decoder         { return gojson.NewDecoder(r) }
func (jsonAPI) Valid(data []byte) bool                         { return gojson.Valid(data) }

// json is a drop-in replacement for encoding/json using goccy/go-json.
// All files in the proxy package use this variable for JSON operations.
var json jsonAPI
