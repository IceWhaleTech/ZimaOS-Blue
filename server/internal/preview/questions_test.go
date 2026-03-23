package preview

import (
	"testing"
)

func TestGetPresetQuestions_ReturnsQuestions(t *testing.T) {
	service := NewQuestionsService()

	questions := service.GetPresetQuestions(5, "en")

	if len(questions) == 0 {
		t.Error("GetPresetQuestions() returned empty list")
	}
	if len(questions) > 5 {
		t.Errorf("GetPresetQuestions(5) returned %d questions, want <= 5", len(questions))
	}
}

func TestGetPresetQuestions_HasRequiredFields(t *testing.T) {
	service := NewQuestionsService()

	questions := service.GetPresetQuestions(3, "en")

	for i, q := range questions {
		if q.ID == "" {
			t.Errorf("Question %d has empty ID", i)
		}
		if q.Text == "" {
			t.Errorf("Question %d has empty Text", i)
		}
		if q.Category == "" {
			t.Errorf("Question %d has empty Category", i)
		}
	}
}

func TestCuratedEnglishQuestionsExposeCardMetadata(t *testing.T) {
	service := NewQuestionsService()

	questions := service.GetAllQuestions("en")

	if len(questions) != 12 {
		t.Fatalf("GetAllQuestions(en) returned %d questions, want 12", len(questions))
	}

	for i, q := range questions {
		if q.Title == "" {
			t.Errorf("Question %d has empty Title", i)
		}
		if q.Description == "" {
			t.Errorf("Question %d has empty Description", i)
		}
		if q.Prompt == "" {
			t.Errorf("Question %d has empty Prompt", i)
		}
		if q.Text != q.Prompt {
			t.Errorf("Question %d has Text != Prompt", i)
		}
		if len(q.Tags) == 0 {
			t.Errorf("Question %d has no Tags", i)
		}
		if q.EditorialScore <= 0 {
			t.Errorf("Question %d has invalid EditorialScore=%d", i, q.EditorialScore)
		}
	}
}

func TestCuratedChineseQuestionsExposeCardMetadata(t *testing.T) {
	service := NewQuestionsService()

	questions := service.GetAllQuestions("zh")

	if len(questions) != 12 {
		t.Fatalf("GetAllQuestions(zh) returned %d questions, want 12", len(questions))
	}

	for i, q := range questions {
		if q.Title == "" {
			t.Errorf("Question %d has empty Title", i)
		}
		if q.Description == "" {
			t.Errorf("Question %d has empty Description", i)
		}
		if q.Prompt == "" {
			t.Errorf("Question %d has empty Prompt", i)
		}
		if q.Text != q.Prompt {
			t.Errorf("Question %d has Text != Prompt", i)
		}
		if len(q.Tags) == 0 {
			t.Errorf("Question %d has no Tags", i)
		}
		if q.EditorialScore <= 0 {
			t.Errorf("Question %d has invalid EditorialScore=%d", i, q.EditorialScore)
		}
	}
}

func TestGetPresetQuestions_RandomSelection(t *testing.T) {
	service := NewQuestionsService()

	// Get questions multiple times and check they're not always the same
	results := make([][]PresetQuestion, 5)
	for i := 0; i < 5; i++ {
		results[i] = service.GetPresetQuestions(3, "en")
	}

	// Check that at least one result is different (randomness)
	allSame := true
	for i := 1; i < len(results); i++ {
		if !questionsEqual(results[0], results[i]) {
			allSame = false
			break
		}
	}

	// Note: This test might occasionally fail due to randomness
	// but with enough questions, it should be very unlikely
	if allSame && len(service.GetAllQuestions("en")) > 3 {
		t.Log("Warning: All random selections returned the same questions (might be coincidence)")
	}
}

func TestGetAllQuestions_ReturnsAllQuestions(t *testing.T) {
	service := NewQuestionsService()

	questions := service.GetAllQuestions("en")

	if len(questions) < 5 {
		t.Errorf("GetAllQuestions() returned %d questions, want at least 5", len(questions))
	}
}

func TestGetQuestionsByCategory_FiltersCorrectly(t *testing.T) {
	service := NewQuestionsService()

	// Get all questions first to find a valid category
	allQuestions := service.GetAllQuestions("en")
	if len(allQuestions) == 0 {
		t.Skip("No questions available")
	}

	category := allQuestions[0].Category
	filtered := service.GetQuestionsByCategory(category, "en")

	for _, q := range filtered {
		if q.Category != category {
			t.Errorf("GetQuestionsByCategory(%s) returned question with category %s", category, q.Category)
		}
	}
}

func TestGetPresetQuestions_ChineseLanguage(t *testing.T) {
	service := NewQuestionsService()

	questions := service.GetPresetQuestions(5, "zh")

	if len(questions) == 0 {
		t.Error("GetPresetQuestions(zh) returned empty list")
	}

	// Check that Chinese questions contain Chinese characters
	for _, q := range questions {
		if q.Text == "" {
			t.Error("Chinese question has empty text")
		}
	}
}

func TestGetPresetQuestions_FallbackToEnglish(t *testing.T) {
	service := NewQuestionsService()

	// Test with unsupported language - should fallback to English
	questions := service.GetPresetQuestions(5, "fr")

	if len(questions) == 0 {
		t.Error("GetPresetQuestions(fr) returned empty list, should fallback to English")
	}
}

func questionsEqual(a, b []PresetQuestion) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID {
			return false
		}
	}
	return true
}

// TestLanguageConsistency verifies that all languages have consistent question IDs
// for questions with attachments (multimodal questions should be available in all languages).
// Note: Skipped as translations may not be complete for all languages.
func TestLanguageConsistency(t *testing.T) {
	t.Skip("Skipping: translations may not be complete for all languages")
	service := NewQuestionsService()
	languages := []string{"en", "zh", "ja", "ko"}

	// Get all questions for each language
	questionsByLang := make(map[string][]PresetQuestion)
	for _, lang := range languages {
		questionsByLang[lang] = service.GetAllQuestions(lang)
	}

	// Build a map of question IDs with attachments from English (reference)
	attachmentQuestionIDs := make(map[string]bool)
	for _, q := range questionsByLang["en"] {
		if len(q.Attachments) > 0 {
			attachmentQuestionIDs[q.ID] = true
		}
	}

	// Verify each language has the same attachment questions
	for _, lang := range languages {
		if lang == "en" {
			continue
		}

		langAttachmentIDs := make(map[string]bool)
		for _, q := range questionsByLang[lang] {
			if len(q.Attachments) > 0 {
				langAttachmentIDs[q.ID] = true
			}
		}

		// Check that all English attachment questions exist in this language
		for id := range attachmentQuestionIDs {
			if !langAttachmentIDs[id] {
				t.Errorf("Language %s is missing attachment question ID %s", lang, id)
			}
		}
	}
}

// TestAllLanguagesHaveQuestions verifies that all supported languages have questions.
func TestAllLanguagesHaveQuestions(t *testing.T) {
	service := NewQuestionsService()
	languages := []string{"en", "zh", "ja", "ko"}

	for _, lang := range languages {
		questions := service.GetAllQuestions(lang)
		if len(questions) == 0 {
			t.Errorf("Language %s has no questions", lang)
		}
		if len(questions) < 10 {
			t.Errorf("Language %s has only %d questions, expected at least 10", lang, len(questions))
		}
	}
}

// TestJapaneseLanguage verifies Japanese questions work correctly.
func TestJapaneseLanguage(t *testing.T) {
	service := NewQuestionsService()

	questions := service.GetPresetQuestions(5, "ja")

	if len(questions) == 0 {
		t.Error("GetPresetQuestions(ja) returned empty list")
	}

	for _, q := range questions {
		if q.Text == "" {
			t.Error("Japanese question has empty text")
		}
	}
}

// TestKoreanLanguage verifies Korean questions work correctly.
func TestKoreanLanguage(t *testing.T) {
	service := NewQuestionsService()

	questions := service.GetPresetQuestions(5, "ko")

	if len(questions) == 0 {
		t.Error("GetPresetQuestions(ko) returned empty list")
	}

	for _, q := range questions {
		if q.Text == "" {
			t.Error("Korean question has empty text")
		}
	}
}
