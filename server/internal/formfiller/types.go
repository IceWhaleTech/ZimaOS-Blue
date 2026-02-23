// Package formfiller provides intelligent form filling capabilities.
package formfiller

import (
	"time"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// FieldType represents the type of a form field.
type FieldType string

const (
	FieldFirstName       FieldType = "firstName"
	FieldLastName        FieldType = "lastName"
	FieldFullName        FieldType = "fullName"
	FieldEmail           FieldType = "email"
	FieldPhone           FieldType = "phone"
	FieldAddress         FieldType = "address"
	FieldCity            FieldType = "city"
	FieldState           FieldType = "state"
	FieldZipCode         FieldType = "zipCode"
	FieldCountry         FieldType = "country"
	FieldUsername        FieldType = "username"
	FieldPassword        FieldType = "password"
	FieldConfirmPassword FieldType = "confirmPassword"
	FieldBirthDate       FieldType = "birthDate"
	FieldCompany         FieldType = "company"
	FieldTitle           FieldType = "title"
	FieldCustom          FieldType = "custom"
)

// DetectionSource indicates how a field was detected.
type DetectionSource string

const (
	DetectionAttribute DetectionSource = "attribute"
	DetectionLabel     DetectionSource = "label"
	DetectionPattern   DetectionSource = "pattern"
	DetectionLLM       DetectionSource = "llm"
)

// FillTemplate represents a fill template with field values.
type FillTemplate struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	IsDefault bool              `json:"is_default"`
	Fields    map[string]string `json:"fields"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// FieldPatterns represents field detection patterns.
type FieldPatterns struct {
	Patterns  map[FieldType][]string `json:"patterns"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// SiteMapping represents site-specific field mappings.
type SiteMapping struct {
	Domain          string            `json:"domain"`
	FieldMappings   map[string]string `json:"field_mappings"` // selector -> fieldType
	LastUsed        time.Time         `json:"last_used"`
	UserCorrections int               `json:"user_corrections"`
}

// DetectedField represents a detected form field.
type DetectedField struct {
	Selector        string          `json:"selector"`
	FieldType       FieldType       `json:"field_type"`
	Confidence      float64         `json:"confidence"`
	DetectionSource DetectionSource `json:"detection_source"`
	SuggestedValue  string          `json:"suggested_value,omitempty"`
	Attributes      FieldAttributes `json:"attributes"`
}

// FieldAttributes contains HTML attributes of a form field.
type FieldAttributes struct {
	ID           string `json:"id,omitempty"`
	Name         string `json:"name,omitempty"`
	Type         string `json:"type,omitempty"`
	Placeholder  string `json:"placeholder,omitempty"`
	Autocomplete string `json:"autocomplete,omitempty"`
	Label        string `json:"label,omitempty"`
}

// DetectRequest represents a field detection request.
type DetectRequest struct {
	HTML       string `json:"html"`
	URL        string `json:"url,omitempty"`
	TemplateID string `json:"template_id,omitempty"`
}

// DetectResponse represents a field detection response.
type DetectResponse struct {
	Fields []DetectedField `json:"fields"`
	Count  int             `json:"count"`
}

// CreateTemplateRequest represents a request to create a template.
type CreateTemplateRequest struct {
	Name      string            `json:"name"`
	IsDefault bool              `json:"is_default"`
	Fields    map[string]string `json:"fields"`
}

// UpdateTemplateRequest represents a request to update a template.
type UpdateTemplateRequest struct {
	Name      string            `json:"name,omitempty"`
	IsDefault *bool             `json:"is_default,omitempty"`
	Fields    map[string]string `json:"fields,omitempty"`
}

// Config represents the form filler configuration.
type Config struct {
	Enabled bool         `json:"enabled"`
	Widget  WidgetConfig `json:"widget"`
	// Password configuration is handled client-side for security
	Detection DetectionConfig `json:"detection"`
	Learning  LearningConfig  `json:"learning"`
}

// WidgetConfig represents widget configuration.
type WidgetConfig struct {
	DefaultPosition   string `json:"default_position"`
	KeyboardShortcut  string `json:"keyboard_shortcut"`
	AutoShowOnForms   bool   `json:"auto_show_on_forms"`
}

// DetectionConfig represents detection configuration.
type DetectionConfig struct {
	UseLLMFallback      bool    `json:"use_llm_fallback"`
	ConfidenceThreshold float64 `json:"confidence_threshold"`
	ScanIntervalMs      int     `json:"scan_interval_ms"`
}

// LearningConfig represents learning configuration.
type LearningConfig struct {
	Enabled         bool `json:"enabled"`
	SyncEnabled     bool `json:"sync_enabled"`
	MaxSiteMappings int  `json:"max_site_mappings"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Enabled: true,
		Widget: WidgetConfig{
			DefaultPosition:   "bottom-right",
			KeyboardShortcut:  "Ctrl+Shift+F",
			AutoShowOnForms:   true,
		},
		Detection: DetectionConfig{
			UseLLMFallback:      true,
			ConfidenceThreshold: 0.7,
			ScanIntervalMs:      1000,
		},
		Learning: LearningConfig{
			Enabled:         true,
			SyncEnabled:     false,
			MaxSiteMappings: 1000,
		},
	}
}

// DefaultPatterns returns the default field detection patterns.
func DefaultPatterns() *FieldPatterns {
	return &FieldPatterns{
		Patterns: map[FieldType][]string{
			FieldEmail:           {"email", "e-mail", "mail", "邮箱", "電郵", "メール"},
			FieldPhone:           {"phone", "tel", "mobile", "手机", "電話", "携帯"},
			FieldFirstName:       {"firstname", "first_name", "first-name", "fname", "名", "名前"},
			FieldLastName:        {"lastname", "last_name", "last-name", "lname", "姓", "苗字"},
			FieldFullName:        {"name", "fullname", "full_name", "full-name", "姓名", "氏名"},
			FieldAddress:         {"address", "addr", "street", "地址", "住所"},
			FieldCity:            {"city", "town", "城市", "市"},
			FieldState:           {"state", "province", "region", "省", "州"},
			FieldZipCode:         {"zip", "zipcode", "zip_code", "postal", "postcode", "邮编", "郵便番号"},
			FieldCountry:         {"country", "nation", "国家", "国"},
			FieldUsername:        {"username", "user_name", "user-name", "login", "用户名", "ユーザー名"},
			FieldPassword:        {"password", "passwd", "pass", "pwd", "密码", "パスワード"},
			FieldConfirmPassword: {"confirm_password", "password_confirm", "password2", "确认密码", "パスワード確認"},
			FieldBirthDate:       {"birthday", "birth_date", "birthdate", "dob", "生日", "誕生日"},
			FieldCompany:         {"company", "organization", "org", "公司", "会社"},
			FieldTitle:           {"title", "job_title", "position", "职位", "役職"},
		},
		UpdatedAt: timeutil.NowTime(),
	}
}
