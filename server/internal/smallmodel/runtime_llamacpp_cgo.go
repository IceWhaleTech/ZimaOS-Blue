//go:build cgo && (darwin || linux || freebsd)

package smallmodel

/*
#cgo linux LDFLAGS: -ldl
#cgo freebsd LDFLAGS: -ldl
#include <dlfcn.h>
#include <stdlib.h>
#include <string.h>

typedef void (*sm_llama_backend_init_fn)(void);
typedef void (*sm_llama_backend_free_fn)(void);

static char sm_last_err[512];

static void sm_clear_err(void) {
	sm_last_err[0] = '\0';
}

static void sm_store_err(const char *msg) {
	if (!msg) {
		msg = "unknown cgo error";
	}
	size_t n = strlen(msg);
	if (n > sizeof(sm_last_err) - 1) {
		n = sizeof(sm_last_err) - 1;
	}
	memcpy(sm_last_err, msg, n);
	sm_last_err[n] = '\0';
}

static const char *sm_get_err(void) {
	if (sm_last_err[0] == '\0') {
		return NULL;
	}
	return sm_last_err;
}

static int sm_llama_probe_symbols(const char *libpath, int call_init_free) {
	sm_clear_err();
	if (!libpath || libpath[0] == '\0') {
		sm_store_err("empty llama library path");
		return 0;
	}

	void *h = dlopen(libpath, RTLD_NOW | RTLD_GLOBAL);
	if (!h) {
		sm_store_err(dlerror());
		return 0;
	}

	dlerror();
	sm_llama_backend_init_fn init_fn = (sm_llama_backend_init_fn) dlsym(h, "llama_backend_init");
	const char *sym_err = dlerror();
	if (sym_err || !init_fn) {
		sm_store_err(sym_err ? sym_err : "missing symbol llama_backend_init");
		dlclose(h);
		return 0;
	}

	dlerror();
	sm_llama_backend_free_fn free_fn = (sm_llama_backend_free_fn) dlsym(h, "llama_backend_free");
	sym_err = dlerror();
	if (sym_err || !free_fn) {
		sm_store_err(sym_err ? sym_err : "missing symbol llama_backend_free");
		dlclose(h);
		return 0;
	}

	if (call_init_free != 0) {
		init_fn();
		free_fn();
	}
	dlclose(h);
	return 1;
}
*/
import "C"

import (
	"fmt"
	"os"
	"strings"
	"unsafe"
)

func ensureLlamaCppCGOBackend() error {
	_, libFile, err := resolveLlamaCppSharedLibFile("llama")
	if err != nil {
		return err
	}
	if libFile == "" {
		return fmt.Errorf("llama cgo probe failed: empty library path")
	}
	cLibPath := C.CString(libFile)
	defer C.free(unsafe.Pointer(cLibPath))

	callInitFree := 0
	switch strings.ToLower(strings.TrimSpace(os.Getenv("SMALL_MODEL_LLAMA_CGO_PROBE_INIT"))) {
	case "1", "true", "yes", "on":
		callInitFree = 1
	}
	if ok := C.sm_llama_probe_symbols(cLibPath, C.int(callInitFree)); ok == 0 {
		detail := "unknown cgo probe failure"
		if cErr := C.sm_get_err(); cErr != nil {
			detail = C.GoString(cErr)
		}
		return fmt.Errorf("llama cgo probe failed for %q: %s", libFile, detail)
	}
	return nil
}
