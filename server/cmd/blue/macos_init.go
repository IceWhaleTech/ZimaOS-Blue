//go:build darwin

package main

import "runtime"

func init() {
	// Pin the main goroutine to the startup thread (thread 0 on macOS).
	// Go docs: "All init functions are run on the startup thread. Calling
	// LockOSThread from an init function will cause the main function to
	// be invoked on that thread."
	//
	// This is required for AppKit: [NSApp run] must execute on pthread
	// main thread (thread 0). We keep thread 0 running the Cocoa event
	// loop for the entire process lifetime, and run the Go server on
	// a separate goroutine.
	runtime.LockOSThread()
}
