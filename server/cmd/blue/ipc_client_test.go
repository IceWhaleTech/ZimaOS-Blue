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

func TestInjectIPCContextParams_AddsBlueUserID(t *testing.T) {
	params := map[string]string{
		"query": "hello",
	}
	getenv := func(key string) string {
		if key == "BLUE_USER_ID" {
			return "user-123"
		}
		return ""
	}

	injectIPCContextParams(params, getenv)

	if got := params["__blue_user_id"]; got != "user-123" {
		t.Fatalf("__blue_user_id = %q, want %q", got, "user-123")
	}
	if got := params["query"]; got != "hello" {
		t.Fatalf("query = %q, want %q", got, "hello")
	}
}

func TestInjectIPCContextParams_DoesNotOverrideExplicitValue(t *testing.T) {
	params := map[string]string{
		"__blue_user_id": "explicit",
	}
	getenv := func(key string) string {
		if key == "BLUE_USER_ID" {
			return "from-env"
		}
		return ""
	}

	injectIPCContextParams(params, getenv)

	if got := params["__blue_user_id"]; got != "explicit" {
		t.Fatalf("__blue_user_id = %q, want %q", got, "explicit")
	}
}

func TestInjectIPCContextParams_AddsSessionID(t *testing.T) {
	params := map[string]string{
		"query": "hello",
	}
	getenv := func(key string) string {
		switch key {
		case "BLUE_USER_ID":
			return "user-123"
		case "BLUE_SESSION_ID":
			return "conv-456"
		default:
			return ""
		}
	}

	injectIPCContextParams(params, getenv)

	if got := params["session_id"]; got != "conv-456" {
		t.Fatalf("session_id = %q, want %q", got, "conv-456")
	}
	if got := params["__blue_user_id"]; got != "user-123" {
		t.Fatalf("__blue_user_id = %q, want %q", got, "user-123")
	}
}

func TestInjectIPCContextParams_DoesNotOverrideExplicitSessionID(t *testing.T) {
	params := map[string]string{
		"session_id": "explicit-conv",
	}
	getenv := func(key string) string {
		if key == "BLUE_SESSION_ID" {
			return "from-env"
		}
		return ""
	}

	injectIPCContextParams(params, getenv)

	if got := params["session_id"]; got != "explicit-conv" {
		t.Fatalf("session_id = %q, want %q", got, "explicit-conv")
	}
}
