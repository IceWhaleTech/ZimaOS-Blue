package querylang

import (
	"encoding/json"
	"math"
	"os"
	"sync"
	"testing"
)

func TestLanguageIDMatchesNativeFastText(t *testing.T) {
	data, err := os.ReadFile("testdata/language-id-native.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Query       string                 `json:"query"`
		Predictions []languageIDPrediction `json:"predictions"`
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	model, err := loadLanguageIDModel(languageIDModelData)
	if err != nil {
		t.Fatal(err)
	}
	if len(model.labels) != 176 {
		t.Fatalf("labels=%d", len(model.labels))
	}
	for _, fixture := range fixtures {
		got := model.predict(fixture.Query)
		for i, want := range fixture.Predictions {
			if got[i].Language != want.Language || math.Abs(got[i].Probability-want.Probability) > 0.00002 {
				t.Errorf("%q prediction %d: got=%+v want=%+v", fixture.Query, i, got[i], want)
			}
		}
	}
}

func TestLanguageIDRejectsUnpinnedModel(t *testing.T) {
	for _, data := range [][]byte{nil, []byte("not a model"), languageIDModelData[:100], append(append([]byte{}, languageIDModelData...), 0)} {
		if _, err := loadLanguageIDModel(data); err == nil {
			t.Fatal("accepted modified or truncated language model")
		}
	}
	corrupt := append([]byte{}, languageIDModelData...)
	corrupt[len(corrupt)-1] ^= 1
	if _, err := loadLanguageIDModel(corrupt); err == nil {
		t.Fatal("accepted corrupt language model")
	}
}

func TestLanguageIDInventoryAndConcurrentDetection(t *testing.T) {
	m, err := builtinLanguageID()
	if err != nil {
		t.Fatal(err)
	}
	labels := map[string]bool{}
	for _, label := range m.labels {
		if label == "no" {
			label = "nb"
		}
		labels[label] = true
	}
	for _, language := range []string{"en", "zh", "ja", "ko", "fr", "de", "es", "pt", "ru", "ar"} {
		if !labels[language] {
			t.Errorf("missing detector label %s", language)
		}
	}
	var group sync.WaitGroup
	for range 32 {
		group.Add(1)
		go func() {
			defer group.Done()
			for range 10 {
				if got := DetectQueryLanguage("biele kvety vedľa jazera"); got != "sk" {
					t.Errorf("concurrent detector=%q", got)
				}
			}
		}()
	}
	group.Wait()
}

func BenchmarkLanguageID(b *testing.B) {
	m, err := loadLanguageIDModel(languageIDModelData)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		m.predict("fotografie psov na pláži")
	}
}

func BenchmarkLanguageIDLoad(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := loadLanguageIDModel(languageIDModelData); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDetectQueryLanguage(b *testing.B) {
	DetectQueryLanguage("biele kvety vedľa jazera")
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		DetectQueryLanguage("biele kvety vedľa jazera")
	}
}
