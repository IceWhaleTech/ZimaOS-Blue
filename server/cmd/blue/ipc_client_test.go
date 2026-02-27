package main

import "testing"

func TestParseIPCArgs_AggregatesRepeatedOptionParams(t *testing.T) {
	params, positional := parseIPCArgs([]string{
		`question=pick`,
		`option=A`,
		`option=B`,
		`option=C`,
	})

	if len(positional) != 0 {
		t.Fatalf("unexpected positional args: %#v", positional)
	}
	if got := params["option"]; got != `["A","B","C"]` {
		t.Fatalf("option param = %q, want %q", got, `["A","B","C"]`)
	}
}

func TestParseIPCArgs_RepeatedNonListKeyKeepsLastValue(t *testing.T) {
	params, _ := parseIPCArgs([]string{
		`question=first`,
		`question=second`,
	})

	if got := params["question"]; got != "second" {
		t.Fatalf("question param = %q, want %q", got, "second")
	}
}

func TestParseIPCArgs_OptionWithCommaIsPreservedInJSONArray(t *testing.T) {
	params, _ := parseIPCArgs([]string{
		`option=A, with comma`,
		`option=B`,
	})
	if got := params["option"]; got != `["A, with comma","B"]` {
		t.Fatalf("option param = %q, want %q", got, `["A, with comma","B"]`)
	}
}
