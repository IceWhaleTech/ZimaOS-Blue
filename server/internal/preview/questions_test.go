package preview

import "testing"

func TestQuestionsService_GetPresetQuestionsReturnsCanonicalCatalog(t *testing.T) {
	service := NewQuestionsService()

	questions := service.GetPresetQuestions(12, "en")
	if len(questions) != 12 {
		t.Fatalf("GetPresetQuestions() returned %d questions, want 12", len(questions))
	}
	if questions[0].ID != "memory-bank" {
		t.Fatalf("GetPresetQuestions()[0].ID = %q, want %q", questions[0].ID, "memory-bank")
	}
	if questions[11].ID != "aristotle-dialogue" {
		t.Fatalf("GetPresetQuestions()[11].ID = %q, want %q", questions[11].ID, "aristotle-dialogue")
	}
}

func TestQuestionsService_GetPresetQuestionPageReturnsExpectedFirstPage(t *testing.T) {
	service := NewQuestionsService()

	questions, total := service.GetPresetQuestionPage(0, 4, "en")
	if len(questions) != 4 {
		t.Fatalf("GetPresetQuestionPage() returned %d questions, want 4", len(questions))
	}
	if total != 12 {
		t.Fatalf("GetPresetQuestionPage() total = %d, want 12", total)
	}
	if questions[0].ID != "memory-bank" {
		t.Fatalf("GetPresetQuestionPage()[0].ID = %q, want %q", questions[0].ID, "memory-bank")
	}
	if questions[3].ID != "content-planner" {
		t.Fatalf("GetPresetQuestionPage()[3].ID = %q, want %q", questions[3].ID, "content-planner")
	}
}

func TestQuestionsService_GetPresetQuestionPageSupportsOffsets(t *testing.T) {
	service := NewQuestionsService()

	questions, total := service.GetPresetQuestionPage(4, 8, "zh")
	if len(questions) != 8 {
		t.Fatalf("GetPresetQuestionPage() returned %d questions, want 8", len(questions))
	}
	if total != 12 {
		t.Fatalf("GetPresetQuestionPage() total = %d, want 12", total)
	}
	if questions[0].ID != "chip-market-briefing" {
		t.Fatalf("GetPresetQuestionPage()[0].ID = %q, want %q", questions[0].ID, "chip-market-briefing")
	}
	if questions[7].ID != "aristotle-dialogue" {
		t.Fatalf("GetPresetQuestionPage()[7].ID = %q, want %q", questions[7].ID, "aristotle-dialogue")
	}
}

func TestQuestionsService_GetAllQuestionsReturnsLocalizedOrFallbackContent(t *testing.T) {
	service := NewQuestionsService()

	questions := service.GetAllQuestions("zh")
	if len(questions) != 12 {
		t.Fatalf("GetAllQuestions(zh) returned %d questions, want 12", len(questions))
	}
	if questions[0].Title != "数字记忆永生银行" {
		t.Fatalf("GetAllQuestions(zh)[0].Title = %q, want %q", questions[0].Title, "数字记忆永生银行")
	}

	fallbackQuestions := service.GetAllQuestions("fr")
	if len(fallbackQuestions) != 12 {
		t.Fatalf("GetAllQuestions(fr) returned %d questions, want 12", len(fallbackQuestions))
	}
	if fallbackQuestions[0].Title != "Digital Memory Bank" {
		t.Fatalf(
			"GetAllQuestions(fr)[0].Title = %q, want %q",
			fallbackQuestions[0].Title,
			"Digital Memory Bank",
		)
	}
}

func TestQuestionsService_GetQuestionsByCategoryFiltersCanonicalCatalog(t *testing.T) {
	service := NewQuestionsService()

	questions := service.GetQuestionsByCategory("product-design", "en")
	if len(questions) != 2 {
		t.Fatalf("GetQuestionsByCategory() returned %d questions, want 2", len(questions))
	}
	if questions[0].ID != "design-adaptation" {
		t.Fatalf("GetQuestionsByCategory()[0].ID = %q, want %q", questions[0].ID, "design-adaptation")
	}
	if questions[1].ID != "inspiration-feed" {
		t.Fatalf("GetQuestionsByCategory()[1].ID = %q, want %q", questions[1].ID, "inspiration-feed")
	}
}
