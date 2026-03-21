package main

import "testing"

func TestTryFastCmd_HelpHandledInLauncher(t *testing.T) {
	if handled := tryFastCmd([]string{"help"}); !handled {
		t.Fatal("expected launcher help to be handled directly")
	}
}

func TestTryFastCmd_HelpWithTopicHandledInLauncher(t *testing.T) {
	if handled := tryFastCmd([]string{"help", "reminder"}); !handled {
		t.Fatal("expected launcher help topic to be handled directly")
	}
}

func TestServicePort_DefaultsTo80(t *testing.T) {
	if got := servicePort(cliFlags{}); got != 80 {
		t.Fatalf("servicePort()=%d, want 80", got)
	}
}

func TestServicePort_UsesShiftedDevPort(t *testing.T) {
	if got := servicePort(cliFlags{devMode: true}); got != 8081 {
		t.Fatalf("servicePort(dev)=%d, want 8081", got)
	}
}
