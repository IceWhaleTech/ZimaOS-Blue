//go:build darwin

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// isRunningInTerminal checks if the current process is running inside
// Terminal.app (or iTerm2, etc.) vs an IDE's integrated terminal.
func isRunningInTerminal() bool {
	tp := os.Getenv("TERM_PROGRAM")
	switch tp {
	case "Apple_Terminal", "iTerm.app", "WarpTerminal", "Alacritty", "tmux":
		return true
	}
	bundleID := os.Getenv("__CFBundleIdentifier")
	switch bundleID {
	case "com.apple.Terminal", "com.googlecode.iterm2", "dev.warp.Warp-Stable":
		return true
	}
	return false
}

// launchViaTerminal opens Terminal.app and runs bluecli inside it using
// NSAppleScript via purego (no osascript subprocess, no CGo).
func launchViaTerminal(cliPath string, args []string) {
	if err := launchViaAppleScript(cliPath, args); err != nil {
		log.Printf("[launcher] NSAppleScript failed: %v, falling back to direct exec", err)
		env := os.Environ()
		syscall.Exec(cliPath, append([]string{cliPath}, args...), env)
	}
}

func launchViaAppleScript(cliPath string, args []string) error {
	// Load Foundation framework
	_, err := purego.Dlopen("/System/Library/Frameworks/Foundation.framework/Foundation", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
	if err != nil {
		return fmt.Errorf("dlopen Foundation: %w", err)
	}

	// Ensure absolute path — Terminal.app opens in ~/ by default
	absCli, err := filepath.Abs(cliPath)
	if err != nil {
		absCli = cliPath
	}

	// Build shell command with proper quoting
	cmdParts := []string{shellQuote(absCli)}
	for _, a := range args {
		cmdParts = append(cmdParts, shellQuote(a))
	}
	cmdStr := strings.Join(cmdParts, " ")

	// AppleScript source — tell Terminal.app to run the command
	scriptSrc := fmt.Sprintf(`tell application "Terminal"
	activate
	do script "%s"
end tell`, strings.ReplaceAll(cmdStr, `"`, `\"`))

	// Create NSString from script source
	nsStrCls := objc.ID(objc.GetClass("NSString"))
	if nsStrCls == 0 {
		return fmt.Errorf("NSString class not found")
	}
	selStringWithUTF8 := objc.RegisterName("stringWithUTF8String:")
	srcBytes := append([]byte(scriptSrc), 0)
	nsSource := nsStrCls.Send(selStringWithUTF8, uintptr(unsafe.Pointer(&srcBytes[0])))
	if nsSource == 0 {
		return fmt.Errorf("failed to create NSString")
	}

	// Create NSAppleScript and execute
	asClass := objc.ID(objc.GetClass("NSAppleScript"))
	if asClass == 0 {
		return fmt.Errorf("NSAppleScript class not found")
	}
	selAlloc := objc.RegisterName("alloc")
	selInitWithSource := objc.RegisterName("initWithSource:")
	selExecuteAndReturnError := objc.RegisterName("executeAndReturnError:")
	selRelease := objc.RegisterName("release")

	script := asClass.Send(selAlloc).Send(selInitWithSource, nsSource)
	if script == 0 {
		return fmt.Errorf("failed to create NSAppleScript")
	}

	return withDarwinOwnedObjectRelease(
		script,
		func(id objc.ID) { id.Send(selRelease) },
		func(id objc.ID) error {
			// executeAndReturnError: takes a pointer to NSDictionary* (error info)
			var errDict uintptr
			id.Send(selExecuteAndReturnError, uintptr(unsafe.Pointer(&errDict)))
			if errDict != 0 {
				return fmt.Errorf("NSAppleScript execution error")
			}
			return nil
		},
	)
}

func withDarwinOwnedObjectRelease(id objc.ID, release func(objc.ID), run func(objc.ID) error) error {
	if run == nil {
		return nil
	}
	if id != 0 && release != nil {
		defer release(id)
	}
	return run(id)
}
