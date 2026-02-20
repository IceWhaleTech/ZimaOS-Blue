//go:build darwin

package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"os"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/speech"
)

// sttAuthResult stores the result of RequestSTTAuthorizationEmbedded
// so runServer can read it later.
var (
	sttAuthStatus int = -1
	sttAuthError  string
)

//export BlueRequestSTTAuthorization
func BlueRequestSTTAuthorization() C.int {
	status, err := speech.RequestSTTAuthorizationEmbedded()
	sttAuthStatus = status
	if err != nil {
		sttAuthError = err.Error()
		fmt.Fprintf(os.Stderr, "[bluelib] STT authorization error: %v\n", err)
		return C.int(status)
	}
	fmt.Fprintf(os.Stderr, "[bluelib] STT authorization status: %d\n", status)
	return C.int(status)
}
