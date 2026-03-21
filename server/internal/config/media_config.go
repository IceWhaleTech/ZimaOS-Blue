package config

import "time"

// MediaConfig holds runtime media-generation settings.
type MediaConfig struct {
	Fallback MediaFallbackConfig `yaml:"fallback" json:"fallback"`
}

// MediaFallbackConfig controls the built-in no-key fallback pipeline.
type MediaFallbackConfig struct {
	Enabled             bool                             `yaml:"enabled" json:"enabled"`
	SearchProviderChain []string                         `yaml:"search_provider_chain" json:"search_provider_chain"`
	SearchMaxResults    int                              `yaml:"search_max_results" json:"search_max_results"`
	ScreenshotWidth     int                              `yaml:"screenshot_width" json:"screenshot_width"`
	ScreenshotHeight    int                              `yaml:"screenshot_height" json:"screenshot_height"`
	ComplexPromptChars  int                              `yaml:"complex_prompt_chars" json:"complex_prompt_chars"`
	PublicSpaces        []MediaFallbackPublicSpaceConfig `yaml:"public_spaces" json:"public_spaces"`
}

// MediaFallbackPublicSpaceConfig describes a browser-driven public creative space preset.
type MediaFallbackPublicSpaceConfig struct {
	ID                      string                              `yaml:"id" json:"id"`
	DisplayName             string                              `yaml:"display_name" json:"display_name"`
	URL                     string                              `yaml:"url" json:"url"`
	Categories              []string                            `yaml:"categories" json:"categories"`
	ReadySelectors          []string                            `yaml:"ready_selectors,omitempty" json:"ready_selectors,omitempty"`
	PromptSelectors         []string                            `yaml:"prompt_selectors,omitempty" json:"prompt_selectors,omitempty"`
	NegativePromptSelectors []string                            `yaml:"negative_prompt_selectors,omitempty" json:"negative_prompt_selectors,omitempty"`
	UploadSelectors         []string                            `yaml:"upload_selectors,omitempty" json:"upload_selectors,omitempty"`
	SubmitSelectors         []string                            `yaml:"submit_selectors,omitempty" json:"submit_selectors,omitempty"`
	SuccessSelectors        []MediaFallbackResultSelectorConfig `yaml:"success_selectors,omitempty" json:"success_selectors,omitempty"`
	ProcessingSelectors     []string                            `yaml:"processing_selectors,omitempty" json:"processing_selectors,omitempty"`
	ErrorSelectors          []string                            `yaml:"error_selectors,omitempty" json:"error_selectors,omitempty"`
	PollInterval            time.Duration                       `yaml:"poll_interval" json:"poll_interval"`
	Timeout                 time.Duration                       `yaml:"timeout" json:"timeout"`
}

// MediaFallbackResultSelectorConfig extracts a finished asset from a public space page.
type MediaFallbackResultSelectorConfig struct {
	Selectors []string `yaml:"selectors" json:"selectors"`
	Attribute string   `yaml:"attribute" json:"attribute"`
	Kind      string   `yaml:"kind" json:"kind"`
}

// DefaultMediaConfig returns the default runtime media configuration.
func DefaultMediaConfig() *MediaConfig {
	return &MediaConfig{
		Fallback: MediaFallbackConfig{
			Enabled:             true,
			SearchProviderChain: []string{"duckduckgo", "bing"},
			SearchMaxResults:    6,
			ScreenshotWidth:     1280,
			ScreenshotHeight:    896,
			ComplexPromptChars:  500,
			PublicSpaces: []MediaFallbackPublicSpaceConfig{
				{
					ID:              "hf-wan21",
					DisplayName:     "Hugging Face Wan 2.1",
					URL:             "https://huggingface.co/spaces/Wan-AI/Wan2.1",
					Categories:      []string{"t2i", "t2v", "i2v", "kf2v"},
					ReadySelectors:  []string{".gradio-container", "textarea", "input[type='text']"},
					PromptSelectors: []string{"textarea", "input[placeholder*='prompt']", "input[type='text']"},
					UploadSelectors: []string{"input[type='file']"},
					SubmitSelectors: []string{"button.primary", "button[type='submit']", "button"},
					SuccessSelectors: []MediaFallbackResultSelectorConfig{
						{Selectors: []string{"video source", "video", "a[href$='.mp4']", "a[download]"}, Attribute: "src", Kind: "video"},
						{Selectors: []string{"img", "a[href$='.png']", "a[href$='.jpg']", "a[download]"}, Attribute: "src", Kind: "image"},
					},
					ProcessingSelectors: []string{".loading", ".generating", "[data-testid='loading']"},
					ErrorSelectors:      []string{".error", ".toast-error", "[role='alert']"},
					PollInterval:        4 * time.Second,
					Timeout:             4 * time.Minute,
				},
				{
					ID:              "modelscope-wan21",
					DisplayName:     "ModelScope Wan 2.1",
					URL:             "https://modelscope.cn/studios/Wan-AI/Wan-2.1",
					Categories:      []string{"t2i", "t2v", "i2v", "kf2v"},
					ReadySelectors:  []string{".studio-app", "textarea", "input[type='text']"},
					PromptSelectors: []string{"textarea", "input[placeholder*='prompt']", "input[type='text']"},
					UploadSelectors: []string{"input[type='file']"},
					SubmitSelectors: []string{"button.primary", "button[type='submit']", "button"},
					SuccessSelectors: []MediaFallbackResultSelectorConfig{
						{Selectors: []string{"video source", "video", "a[href$='.mp4']", "a[download]"}, Attribute: "src", Kind: "video"},
						{Selectors: []string{"img", "a[href$='.png']", "a[href$='.jpg']", "a[download]"}, Attribute: "src", Kind: "image"},
					},
					ProcessingSelectors: []string{".loading", ".generating", ".ant-spin", "[data-loading='true']"},
					ErrorSelectors:      []string{".error", ".ant-alert", "[role='alert']"},
					PollInterval:        4 * time.Second,
					Timeout:             4 * time.Minute,
				},
				{
					ID:              "modelscope-scepter",
					DisplayName:     "ModelScope Scepter Studio",
					URL:             "https://modelscope.cn/studios/iic/scepter_studio/summary",
					Categories:      []string{"t2i", "i2i"},
					ReadySelectors:  []string{".studio-app", "textarea", "input[type='text']"},
					PromptSelectors: []string{"textarea", "input[placeholder*='prompt']", "input[type='text']"},
					UploadSelectors: []string{"input[type='file']"},
					SubmitSelectors: []string{"button.primary", "button[type='submit']", "button"},
					SuccessSelectors: []MediaFallbackResultSelectorConfig{
						{Selectors: []string{"img", "a[href$='.png']", "a[href$='.jpg']", "a[download]"}, Attribute: "src", Kind: "image"},
					},
					ProcessingSelectors: []string{".loading", ".generating", ".ant-spin", "[data-loading='true']"},
					ErrorSelectors:      []string{".error", ".ant-alert", "[role='alert']"},
					PollInterval:        4 * time.Second,
					Timeout:             4 * time.Minute,
				},
			},
		},
	}
}
