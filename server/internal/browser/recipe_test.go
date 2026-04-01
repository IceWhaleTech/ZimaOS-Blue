package browser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRecipeRegistry(t *testing.T) {
	r := NewRecipeRegistry()
	require.NotNil(t, r)

	// All 4 built-in recipes should be registered
	infos := r.List()
	assert.Len(t, infos, 4)

	names := make(map[string]bool)
	for _, info := range infos {
		names[info.Name] = true
	}
	assert.True(t, names["search"])
	assert.True(t, names["fill_form"])
	assert.True(t, names["extract"])
	assert.True(t, names["login"])
}

func TestRecipeRegistry_Get(t *testing.T) {
	r := NewRecipeRegistry()

	recipe, ok := r.Get("search")
	assert.True(t, ok)
	assert.Equal(t, "search", recipe.Name())

	_, ok = r.Get("nonexistent")
	assert.False(t, ok)
}

func TestRecipeRegistry_KeepTab(t *testing.T) {
	r := NewRecipeRegistry()

	tests := []struct {
		name    string
		keepTab bool
	}{
		{"search", false},
		{"fill_form", true},
		{"extract", false},
		{"login", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recipe, ok := r.Get(tt.name)
			require.True(t, ok)
			assert.Equal(t, tt.keepTab, recipe.KeepTab())
		})
	}
}

func TestSearchRecipe_Validate(t *testing.T) {
	r := &searchRecipe{}

	tests := []struct {
		name    string
		params  map[string]string
		wantErr bool
	}{
		{
			name:    "valid with query only",
			params:  map[string]string{"query": "test"},
			wantErr: false,
		},
		{
			name:    "valid with engine",
			params:  map[string]string{"query": "test", "engine": "bing"},
			wantErr: false,
		},
		{
			name:    "missing query",
			params:  map[string]string{},
			wantErr: true,
		},
		{
			name:    "unsupported engine",
			params:  map[string]string{"query": "test", "engine": "yahoo"},
			wantErr: true,
		},
		{
			name:    "all supported engines",
			params:  map[string]string{"query": "test", "engine": "google"},
			wantErr: false,
		},
		{
			name:    "baidu engine",
			params:  map[string]string{"query": "test", "engine": "baidu"},
			wantErr: false,
		},
		{
			name:    "duckduckgo engine",
			params:  map[string]string{"query": "test", "engine": "duckduckgo"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.Validate(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBuildSearchURL(t *testing.T) {
	tests := []struct {
		name   string
		engine string
		query  string
		want   string
	}{
		{
			name:   "google",
			engine: "google",
			query:  "OpenAI Responses API",
			want:   "https://www.google.com/search?q=OpenAI+Responses+API",
		},
		{
			name:   "bing",
			engine: "bing",
			query:  "OpenAI Responses API",
			want:   "https://www.bing.com/search?q=OpenAI+Responses+API",
		},
		{
			name:   "duckduckgo",
			engine: "duckduckgo",
			query:  "OpenAI Responses API",
			want:   "https://duckduckgo.com/?q=OpenAI+Responses+API",
		},
		{
			name:   "baidu",
			engine: "baidu",
			query:  "OpenAI Responses API",
			want:   "https://www.baidu.com/s?wd=OpenAI+Responses+API",
		},
		{
			name:   "unknown engine",
			engine: "yahoo",
			query:  "OpenAI Responses API",
			want:   "",
		},
		{
			name:   "empty query",
			engine: "bing",
			query:  "  ",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, buildSearchURL(tt.engine, tt.query))
		})
	}
}

func TestFinalizeSearchRecipeResult_AllowsMissingPageInfo(t *testing.T) {
	result, err := finalizeSearchRecipeResult(nil, "bing", "OpenAI Responses API", "5", []SearchResult{
		{Title: "Docs", URL: "https://example.com/docs", Snippet: "latest docs"},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, "", result.Data["page_url"])
}

func TestFillFormRecipe_Validate(t *testing.T) {
	r := &fillFormRecipe{}

	tests := []struct {
		name    string
		params  map[string]string
		wantErr bool
	}{
		{
			name:    "valid",
			params:  map[string]string{"url": "https://example.com", "fields": `{"name":"John"}`},
			wantErr: false,
		},
		{
			name:    "missing url",
			params:  map[string]string{"fields": `{"name":"John"}`},
			wantErr: true,
		},
		{
			name:    "missing fields",
			params:  map[string]string{"url": "https://example.com"},
			wantErr: true,
		},
		{
			name:    "invalid fields JSON",
			params:  map[string]string{"url": "https://example.com", "fields": "not json"},
			wantErr: true,
		},
		{
			name:    "empty fields object",
			params:  map[string]string{"url": "https://example.com", "fields": `{}`},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.Validate(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExtractRecipe_Validate(t *testing.T) {
	r := &extractRecipe{}

	tests := []struct {
		name    string
		params  map[string]string
		wantErr bool
	}{
		{
			name:    "valid",
			params:  map[string]string{"url": "https://example.com", "selectors": `{"title":"h1"}`},
			wantErr: false,
		},
		{
			name:    "missing url",
			params:  map[string]string{"selectors": `{"title":"h1"}`},
			wantErr: true,
		},
		{
			name:    "missing selectors",
			params:  map[string]string{"url": "https://example.com"},
			wantErr: true,
		},
		{
			name:    "invalid selectors JSON",
			params:  map[string]string{"url": "https://example.com", "selectors": "bad"},
			wantErr: true,
		},
		{
			name:    "empty selectors",
			params:  map[string]string{"url": "https://example.com", "selectors": `{}`},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.Validate(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLoginRecipe_Validate(t *testing.T) {
	r := &loginRecipe{}

	tests := []struct {
		name    string
		params  map[string]string
		wantErr bool
	}{
		{
			name:    "valid",
			params:  map[string]string{"url": "https://example.com", "username": "user", "password": "pass"},
			wantErr: false,
		},
		{
			name:    "missing url",
			params:  map[string]string{"username": "user", "password": "pass"},
			wantErr: true,
		},
		{
			name:    "missing username",
			params:  map[string]string{"url": "https://example.com", "password": "pass"},
			wantErr: true,
		},
		{
			name:    "missing password",
			params:  map[string]string{"url": "https://example.com", "username": "user"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.Validate(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSearchEngineTemplates(t *testing.T) {
	// Verify all search engines have required selectors
	for name, engine := range searchEngines {
		t.Run(name, func(t *testing.T) {
			assert.NotEmpty(t, engine.URL, "URL")
			assert.NotEmpty(t, engine.SearchBox, "SearchBox")
			assert.NotEmpty(t, engine.ResultContainer, "ResultContainer")
			assert.NotEmpty(t, engine.ResultTitle, "ResultTitle")
			assert.NotEmpty(t, engine.ResultLink, "ResultLink")
			assert.NotEmpty(t, engine.ResultSnippet, "ResultSnippet")
		})
	}
}

func TestParseInt(t *testing.T) {
	assert.Equal(t, 10, parseInt("10"))
	assert.Equal(t, 0, parseInt(""))
	assert.Equal(t, 5, parseInt("5abc"))
	assert.Equal(t, 0, parseInt("abc"))
	assert.Equal(t, 123, parseInt("123"))
}

func TestTernary(t *testing.T) {
	assert.Equal(t, "yes", ternary(true, "yes", "no"))
	assert.Equal(t, "no", ternary(false, "yes", "no"))
}

func TestRecipeRegistry_Execute_UnknownRecipe(t *testing.T) {
	r := NewRecipeRegistry()
	_, err := r.Execute(nil, nil, "nonexistent", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown recipe")
}

func TestRecipeRegistry_Execute_ValidationError(t *testing.T) {
	r := NewRecipeRegistry()
	// search requires "query"
	_, err := r.Execute(nil, nil, "search", map[string]string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query is required")
}
