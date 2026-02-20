//go:build !darwin

package main

/*
#include <stdlib.h>
*/
import "C"

// sttAuthResult stores the result — on non-darwin, always -1 (not applicable).
var (
	sttAuthStatus int = -1
	sttAuthError  string
)

//export BlueRequestSTTAuthorization
func BlueRequestSTTAuthorization() C.int {
	return C.int(-1) // Not applicable on this platform
}
