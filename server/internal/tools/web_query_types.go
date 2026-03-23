package tools

type webQueryMedia struct {
	Kind         string              `json:"kind,omitempty"`
	Platform     string              `json:"platform,omitempty"`
	VideoID      string              `json:"video_id,omitempty"`
	DurationMS   int64               `json:"duration_ms,omitempty"`
	Language     string              `json:"language,omitempty"`
	ThumbnailURL string              `json:"thumbnail_url,omitempty"`
	Author       string              `json:"author,omitempty"`
	PublishedAt  string              `json:"published_at,omitempty"`
	AnalysisMode string              `json:"analysis_mode,omitempty"`
	Summary      string              `json:"summary,omitempty"`
	Items        []webQueryMediaItem `json:"items,omitempty"`
}

type webQueryPage struct {
	Title         string `json:"title,omitempty"`
	Content       string `json:"content,omitempty"`
	ContentFormat string `json:"content_format,omitempty"`
	TargetURL     string `json:"target_url,omitempty"`
	FinalURL      string `json:"final_url,omitempty"`
	Source        string `json:"source,omitempty"`
}

type webQueryTranscript struct {
	Status        string                      `json:"status,omitempty"`
	Source        string                      `json:"source,omitempty"`
	Language      string                      `json:"language,omitempty"`
	Text          string                      `json:"text,omitempty"`
	Segments      []webQueryTranscriptSegment `json:"segments,omitempty"`
	DurationMS    int64                       `json:"duration_ms,omitempty"`
	CoverageRatio float64                     `json:"coverage_ratio,omitempty"`
	Confidence    float64                     `json:"confidence,omitempty"`
	Generated     bool                        `json:"generated,omitempty"`
}

type webQueryTranscriptSegment struct {
	StartMS int64  `json:"start_ms,omitempty"`
	EndMS   int64  `json:"end_ms,omitempty"`
	Text    string `json:"text,omitempty"`
}
