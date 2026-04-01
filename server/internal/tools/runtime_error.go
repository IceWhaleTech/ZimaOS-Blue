package tools

import "strings"

type toolRuntimeError struct {
	code    string
	message string
	details map[string]interface{}
	cause   error
}

func (e *toolRuntimeError) Error() string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e.message)
}

func (e *toolRuntimeError) ToolRuntimeCode() string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e.code)
}

func (e *toolRuntimeError) ToolRuntimeDetails() map[string]interface{} {
	if e == nil {
		return nil
	}
	return cloneJSONInterfaceMap(e.details)
}

func (e *toolRuntimeError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func newToolRuntimeError(code, message string, cause error, details map[string]interface{}) error {
	return &toolRuntimeError{
		code:    strings.TrimSpace(code),
		message: strings.TrimSpace(message),
		details: cloneJSONInterfaceMap(details),
		cause:   cause,
	}
}
