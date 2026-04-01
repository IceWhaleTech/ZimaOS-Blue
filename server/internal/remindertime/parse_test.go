package remindertime

import (
	"testing"
	"time"
)

func withFixedReminderNow(t *testing.T, ts time.Time) {
	t.Helper()
	prev := nowTime
	nowTime = func() time.Time { return ts }
	t.Cleanup(func() {
		nowTime = prev
	})
}

func TestParse_Duration(t *testing.T) {
	before := time.Now()
	got, err := Parse("1h30m")
	after := time.Now()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := before.Add(90 * time.Minute)
	if got.Before(expected.Add(-time.Second)) || got.After(after.Add(90*time.Minute+time.Second)) {
		t.Errorf("Parse(1h30m) = %v, expected ~%v", got, expected)
	}
}

func TestParse_EnglishNaturalDuration(t *testing.T) {
	before := time.Now()
	got, err := Parse("in 10 seconds")
	after := time.Now()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := before.Add(10 * time.Second)
	if got.Before(expected.Add(-time.Second)) || got.After(after.Add(11*time.Second)) {
		t.Errorf("Parse(in 10 seconds) = %v, expected ~%v", got, expected)
	}
}

func TestParse_ChineseNaturalDuration(t *testing.T) {
	before := time.Now()
	got, err := Parse("10秒钟以后")
	after := time.Now()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := before.Add(10 * time.Second)
	if got.Before(expected.Add(-time.Second)) || got.After(after.Add(11*time.Second)) {
		t.Errorf("Parse(10秒钟以后) = %v, expected ~%v", got, expected)
	}
}

func TestParse_ChineseSentenceDuration(t *testing.T) {
	before := time.Now()
	got, err := Parse("提醒我10秒钟以后喝水")
	after := time.Now()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := before.Add(10 * time.Second)
	if got.Before(expected.Add(-time.Second)) || got.After(after.Add(11*time.Second)) {
		t.Errorf("Parse(sentence) = %v, expected ~%v", got, expected)
	}
}

func TestParse_RFC3339(t *testing.T) {
	got, err := Parse("2026-06-15T10:30:00Z")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Year() != 2026 || got.Month() != 6 || got.Day() != 15 {
		t.Errorf("Parse(RFC3339) = %v", got)
	}
}

func TestParse_CommonDateTime(t *testing.T) {
	got, err := Parse("2026-03-01 09:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Year() != 2026 || got.Month() != 3 || got.Day() != 1 || got.Hour() != 9 {
		t.Errorf("Parse(datetime) = %v", got)
	}
}

func TestParse_NaturalAbsoluteCatalanTomorrowTime(t *testing.T) {
	withFixedReminderNow(t, time.Date(2026, 3, 31, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60)))
	got, err := Parse("dema a les 9")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "2026-04-01 09:00"
	if got.Format("2006-01-02 15:04") != want {
		t.Fatalf("Parse(ca-ES tomorrow) = %s, want %s", got.Format("2006-01-02 15:04"), want)
	}
}

func TestParse_NaturalAbsoluteCzechTomorrowTime(t *testing.T) {
	withFixedReminderNow(t, time.Date(2026, 3, 31, 10, 0, 0, 0, time.UTC))
	got, err := Parse("zitra v 9")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "2026-04-01 09:00"
	if got.Format("2006-01-02 15:04") != want {
		t.Fatalf("Parse(cs-CZ tomorrow) = %s, want %s", got.Format("2006-01-02 15:04"), want)
	}
}

func TestInferTimeStringFromMessage_ChineseTomorrowMorning(t *testing.T) {
	withFixedReminderNow(t, time.Date(2026, 3, 31, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60)))
	got, ok := InferTimeStringFromMessage("帮我明早 9 点提醒我发送周报。")
	if !ok {
		t.Fatal("expected inferred time from zh-CN reminder message")
	}
	if got != "2026-04-01 09:00" {
		t.Fatalf("InferTimeStringFromMessage(zh-CN) = %q, want 2026-04-01 09:00", got)
	}
}

func TestInferTimeStringFromMessage_DoesNotGuessFromUnspecifiedTomorrowNumber(t *testing.T) {
	withFixedReminderNow(t, time.Date(2026, 3, 31, 10, 0, 0, 0, time.UTC))
	if got, ok := InferTimeStringFromMessage("Remind me tomorrow to buy 9 apples."); ok {
		t.Fatalf("InferTimeStringFromMessage guessed %q for non-time tomorrow number", got)
	}
}

func TestParse_Invalid(t *testing.T) {
	if _, err := Parse("not-a-time"); err == nil {
		t.Fatal("expected error for invalid format")
	}
}
