package main

import (
	"testing"
)

func TestShouldAutoOpenStartupBrowser(t *testing.T) {
	oldJSON := jsonOutput
	defer func() {
		jsonOutput = oldJSON
	}()

	t.Run("enabled for normal foreground startup", func(t *testing.T) {
		t.Setenv("BLUE_GATEWAY_SUPERVISOR", "")
		jsonOutput = false

		if got := shouldAutoOpenStartupBrowser(); !got {
			t.Fatalf("shouldAutoOpenStartupBrowser() = %v, want true", got)
		}
	})

	t.Run("disabled for supervised gateway worker", func(t *testing.T) {
		t.Setenv("BLUE_GATEWAY_SUPERVISOR", "1")
		jsonOutput = false

		if got := shouldAutoOpenStartupBrowser(); got {
			t.Fatalf("shouldAutoOpenStartupBrowser() = %v, want false", got)
		}
	})

	t.Run("disabled for json output mode", func(t *testing.T) {
		t.Setenv("BLUE_GATEWAY_SUPERVISOR", "")
		jsonOutput = true

		if got := shouldAutoOpenStartupBrowser(); got {
			t.Fatalf("shouldAutoOpenStartupBrowser() = %v, want false", got)
		}
	})
}

func TestRunForegroundServerConfiguresStartupBrowserHandler(t *testing.T) {
	oldRunServerEntry := runServerEntry
	oldSetStartupURLHandler := setStartupURLHandler
	oldOpenStartupBrowserURL := openStartupBrowserURL
	defer func() {
		runServerEntry = oldRunServerEntry
		setStartupURLHandler = oldSetStartupURLHandler
		openStartupBrowserURL = oldOpenStartupBrowserURL
	}()

	var (
		runCalled        bool
		handlerInstalled bool
	)

	runServerEntry = func() error {
		runCalled = true
		return nil
	}
	setStartupURLHandler = func(handler func(string)) {
		handlerInstalled = handler != nil
	}
	openStartupBrowserURL = func(rawURL string) error {
		return nil
	}

	t.Setenv("BLUE_GATEWAY_SUPERVISOR", "")
	jsonOutput = false

	if err := runForegroundServer(); err != nil {
		t.Fatalf("runForegroundServer() error = %v", err)
	}

	if !handlerInstalled {
		t.Fatal("expected runForegroundServer to install a startup browser handler")
	}
	if !runCalled {
		t.Fatal("expected runForegroundServer to invoke runServer")
	}
}

func TestRunForegroundServerSkipsStartupBrowserHandlerWhenAutoOpenDisabled(t *testing.T) {
	oldRunServerEntry := runServerEntry
	oldSetStartupURLHandler := setStartupURLHandler
	defer func() {
		runServerEntry = oldRunServerEntry
		setStartupURLHandler = oldSetStartupURLHandler
	}()

	var (
		runCalled bool
		gotNil    bool
	)

	runServerEntry = func() error {
		runCalled = true
		return nil
	}
	setStartupURLHandler = func(handler func(string)) {
		gotNil = handler == nil
	}

	t.Setenv("BLUE_GATEWAY_SUPERVISOR", "1")
	jsonOutput = false

	if err := runForegroundServer(); err != nil {
		t.Fatalf("runForegroundServer() error = %v", err)
	}

	if !gotNil {
		t.Fatal("expected runForegroundServer to clear startup browser handler when auto-open is disabled")
	}
	if !runCalled {
		t.Fatal("expected runForegroundServer to invoke runServer")
	}
}
