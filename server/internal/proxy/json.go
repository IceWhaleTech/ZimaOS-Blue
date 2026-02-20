package proxy

import jsoniter "github.com/json-iterator/go"

// json is a drop-in replacement for encoding/json using jsoniter.
// ConfigCompatibleWithStandardLibrary ensures identical behavior
// with significantly better performance (fewer allocations, faster parsing).
var json = jsoniter.ConfigCompatibleWithStandardLibrary
