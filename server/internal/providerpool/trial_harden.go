//go:build cgo

package providerpool

/*
#cgo CFLAGS: -I${SRCDIR}/harden
#cgo darwin,arm64  LDFLAGS: -L${SRCDIR}/harden/darwin_arm64 -lharden
#cgo darwin,amd64  LDFLAGS: -L${SRCDIR}/harden/darwin_amd64 -lharden
#cgo linux,amd64   LDFLAGS: -L${SRCDIR}/harden/linux_amd64 -lharden
#cgo linux,arm64   LDFLAGS: -L${SRCDIR}/harden/linux_arm64 -lharden
#cgo windows,amd64 LDFLAGS: -L${SRCDIR}/harden/windows_amd64 -lharden
#include "harden.h"
#include <stdlib.h>
*/
import "C"
import "unsafe"

func init() {
	trialPersistHook = &TrialPersistHook{
		Load:           nil,
		Save:           hardenSave,
		CheckActivated: hardenCheckActivated,
	}
}

func hardenSave(dataDir string, licenseSig []byte, claims *LicenseClaims, state *trialState) error {
	cd := C.CString(dataDir)
	defer C.free(unsafe.Pointer(cd))
	var sp *C.uchar
	if len(licenseSig) > 0 {
		sp = (*C.uchar)(unsafe.Pointer(&licenseSig[0]))
	}
	C.harden_mark(cd, sp, C.int(len(licenseSig)))
	return nil
}

func hardenCheckActivated(dataDir string, licenseSig []byte, claims *LicenseClaims) bool {
	cd := C.CString(dataDir)
	defer C.free(unsafe.Pointer(cd))
	var sp *C.uchar
	if len(licenseSig) > 0 {
		sp = (*C.uchar)(unsafe.Pointer(&licenseSig[0]))
	}
	return C.harden_check(cd, sp, C.int(len(licenseSig))) != 0
}
