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
