package mediagen

import (
	"testing"
)

func TestClassifyMediaIntent_English(t *testing.T) {
	tests := []struct {
		name      string
		msg       string
		hasImages bool
		imgCount  int
		wantCat   MediaCategory
		wantNil   bool
		minConf   float64
	}{
		// --- T2I ---
		{"t2i basic", "generate an image of a cat", false, 0, CategoryT2I, false, 0.8},
		{"t2i draw", "draw me a picture of a sunset", false, 0, CategoryT2I, false, 0.8},
		{"t2i create", "create a photo of mountains", false, 0, CategoryT2I, false, 0.8},
		{"t2i paint", "paint a portrait of a woman", false, 0, CategoryT2I, false, 0.8},
		{"t2i make", "make an illustration of a dragon", false, 0, CategoryT2I, false, 0.8},
		{"t2i wallpaper", "produce a wallpaper with stars", false, 0, CategoryT2I, false, 0.8},
		// --- T2V ---
		{"t2v basic", "generate a video of waves", false, 0, CategoryT2V, false, 0.8},
		{"t2v create", "create an animation of a dancing cat", false, 0, CategoryT2V, false, 0.8},
		{"t2v make", "make a clip of fireworks", false, 0, CategoryT2V, false, 0.8},
		// --- I2V ---
		{"i2v animate", "animate this image", true, 1, CategoryI2V, false, 0.85},
		{"i2v make move", "make it move", true, 1, CategoryI2V, false, 0.85},
		{"i2v video", "create a video from this", true, 1, CategoryI2V, false, 0.8},
		// --- I2I ---
		{"i2i edit", "edit this image", true, 1, CategoryI2I, false, 0.85},
		{"i2i modify", "modify the background", true, 1, CategoryI2I, false, 0.85},
		{"i2i remove", "remove the person from this photo", true, 1, CategoryI2I, false, 0.85},
		{"i2i change", "change the style to watercolor", true, 1, CategoryI2I, false, 0.85},
		{"i2i photoshop", "photoshop this image", true, 1, CategoryI2I, false, 0.85},
		{"i2i touchup", "touch up this photo", true, 1, CategoryI2I, false, 0.85},
		{"i2i crop", "crop this image", true, 1, CategoryI2I, false, 0.85},
		{"i2i sharpen", "sharpen this photo", true, 1, CategoryI2I, false, 0.85},
		{"i2i restore", "restore this old photo", true, 1, CategoryI2I, false, 0.85},
		{"i2i cleanup", "clean up this image", true, 1, CategoryI2I, false, 0.85},
		// --- KF2V ---
		{"kf2v two frames", "create a video from these two frames", true, 2, CategoryKF2V, false, 0.8},
		{"kf2v animate pair", "animate between these images", true, 2, CategoryKF2V, false, 0.8},
		// --- Negation ---
		{"neg dont", "don't generate any images", false, 0, CategoryNone, true, 0},
		{"neg stop", "stop creating videos", false, 0, CategoryNone, true, 0},
		// --- Questions ---
		{"q howto", "how to generate images with AI?", false, 0, CategoryNone, true, 0},
		{"q explain", "can you explain how video generation works?", false, 0, CategoryNone, true, 0},
		// --- Non-media ---
		{"non-media", "what's the weather today?", false, 0, CategoryNone, true, 0},
		{"non-media code", "write a function to sort an array", false, 0, CategoryNone, true, 0},
		{"empty", "", false, 0, CategoryNone, true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyMediaIntent(tt.msg, tt.hasImages, tt.imgCount, "en-US")
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got category=%s conf=%.2f", got.Category, got.Confidence)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected category=%s, got nil", tt.wantCat)
			}
			if got.Category != tt.wantCat {
				t.Errorf("category: got %s, want %s", got.Category, tt.wantCat)
			}
			if got.Confidence < tt.minConf {
				t.Errorf("confidence: got %.2f, want >= %.2f", got.Confidence, tt.minConf)
			}
		})
	}
}

func TestClassifyMediaIntent_Chinese(t *testing.T) {
	tests := []struct {
		name    string
		msg     string
		hasImg  bool
		wantCat MediaCategory
		wantNil bool
	}{
		{"t2i gen", "生成一张猫的图片", false, CategoryT2I, false},
		{"t2i draw", "画一幅山水画", false, CategoryT2I, false},
		{"t2i create", "创建一张海报", false, CategoryT2I, false},
		{"t2v gen", "生成一段海浪的视频", false, CategoryT2V, false},
		{"t2v make", "制作一个动画", false, CategoryT2V, false},
		{"i2v animate", "让它动起来", true, CategoryI2V, false},
		{"i2v video", "变成视频", true, CategoryI2V, false},
		{"i2i edit", "编辑这张图片", true, CategoryI2I, false},
		{"i2i modify", "修改背景", true, CategoryI2I, false},
		{"i2i retouch", "修图", true, CategoryI2I, false},
		{"i2i edit photo", "改图", true, CategoryI2I, false},
		{"i2i p photo", "p图", true, CategoryI2I, false},
		{"i2i cutout", "抠图", true, CategoryI2I, false},
		{"i2i beauty", "美颜", true, CategoryI2I, false},
		{"i2i remove watermark", "去水印", true, CategoryI2I, false},
		{"i2i swap bg", "换背景", true, CategoryI2I, false},
		{"neg", "不要生成图片", false, CategoryNone, true},
		{"q", "怎么生成图片？", false, CategoryNone, true},
		{"non-media", "今天天气怎么样", false, CategoryNone, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyMediaIntent(tt.msg, tt.hasImg, boolToInt(tt.hasImg), "zh-CN")
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got category=%s", got.Category)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected category=%s, got nil", tt.wantCat)
			}
			if got.Category != tt.wantCat {
				t.Errorf("category: got %s, want %s", got.Category, tt.wantCat)
			}
		})
	}
}

func TestClassifyMediaIntent_Japanese(t *testing.T) {
	tests := []struct {
		name    string
		msg     string
		wantCat MediaCategory
		wantNil bool
	}{
		{"t2i", "猫の画像を作成して", CategoryT2I, false},
		{"t2i draw", "風景の絵を描いて", CategoryT2I, false},
		{"t2v", "動画を作って", CategoryT2V, false},
		{"neg", "描かないでください", CategoryNone, true},
		{"q", "どうやって画像を作るの", CategoryNone, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyMediaIntent(tt.msg, false, 0, "ja-JP")
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got category=%s", got.Category)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected category=%s, got nil", tt.wantCat)
			}
			if got.Category != tt.wantCat {
				t.Errorf("category: got %s, want %s", got.Category, tt.wantCat)
			}
		})
	}
}

func TestClassifyMediaIntent_Korean(t *testing.T) {
	tests := []struct {
		name    string
		msg     string
		wantCat MediaCategory
		wantNil bool
	}{
		{"t2i", "고양이 이미지를 만들어줘", CategoryT2I, false},
		{"t2i draw", "그림을 그려줘", CategoryT2I, false},
		{"t2v", "비디오를 만들어", CategoryT2V, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyMediaIntent(tt.msg, false, 0, "ko-KR")
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got category=%s", got.Category)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected category=%s, got nil", tt.wantCat)
			}
			if got.Category != tt.wantCat {
				t.Errorf("category: got %s, want %s", got.Category, tt.wantCat)
			}
		})
	}
}

func TestClassifyMediaIntent_European(t *testing.T) {
	tests := []struct {
		name    string
		msg     string
		locale  string
		wantCat MediaCategory
		wantNil bool
	}{
		// German
		{"de t2i", "erstelle ein bild von einer katze", "de-DE", CategoryT2I, false},
		{"de t2v", "generiere ein video vom meer", "de-DE", CategoryT2V, false},
		{"de neg", "nicht erstellen", "de-DE", CategoryNone, true},
		// French
		{"fr t2i", "créer une image d'un chat", "fr-FR", CategoryT2I, false},
		{"fr t2v", "générer une vidéo de la mer", "fr-FR", CategoryT2V, false},
		// Spanish
		{"es t2i", "crear una imagen de un gato", "es-ES", CategoryT2I, false},
		{"es t2v", "generar un vídeo del mar", "es-ES", CategoryT2V, false},
		// Portuguese
		{"pt t2i", "criar uma imagem de um gato", "pt-BR", CategoryT2I, false},
		// Italian
		{"it t2i", "creare un'immagine di un gatto", "it-IT", CategoryT2I, false},
		// Dutch
		{"nl t2i", "maak een afbeelding van een kat", "nl-NL", CategoryT2I, false},
		// Russian
		{"ru t2i", "создай картинку кота", "ru-RU", CategoryT2I, false},
		{"ru t2v", "сделай видео моря", "ru-RU", CategoryT2V, false},
		{"ru neg", "не надо создавать", "ru-RU", CategoryNone, true},
		// Polish
		{"pl t2i", "stwórz obraz kota", "pl-PL", CategoryT2I, false},
		// Swedish
		{"sv t2i", "skapa en bild av en katt", "sv-SE", CategoryT2I, false},
		// Danish
		{"da t2i", "opret et billede af en kat", "da-DK", CategoryT2I, false},
		// Norwegian
		{"nb t2i", "lag et bilde av en katt", "nb-NO", CategoryT2I, false},
		// Czech
		{"cs t2i", "vytvoř obrázek kočky", "cs-CZ", CategoryT2I, false},
		// Slovak
		{"sk t2i", "vytvor obrázok mačky", "sk-SK", CategoryT2I, false},
		// Hungarian
		{"hu t2i", "készíts egy képet egy macskáról", "hu-HU", CategoryT2I, false},
		// Romanian
		{"ro t2i", "creează o imagine cu o pisică", "ro-RO", CategoryT2I, false},
		// Greek
		{"el t2i", "δημιούργησε μια εικόνα μιας γάτας", "el-GR", CategoryT2I, false},
		// Croatian
		{"hr t2i", "stvori sliku mačke", "hr-HR", CategoryT2I, false},
		// Catalan (maps to Spanish)
		{"ca t2i", "crear una imagen de un gato", "ca-ES", CategoryT2I, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyMediaIntent(tt.msg, false, 0, tt.locale)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got category=%s", got.Category)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected category=%s, got nil", tt.wantCat)
			}
			if got.Category != tt.wantCat {
				t.Errorf("category: got %s, want %s (locale=%s)", got.Category, tt.wantCat, tt.locale)
			}
		})
	}
}

func TestClassifyMediaIntent_MixedLanguage(t *testing.T) {
	// Users often mix English keywords with their native language
	tests := []struct {
		name    string
		msg     string
		locale  string
		wantCat MediaCategory
	}{
		{"zh+en", "generate一张猫的图片", "zh-CN", CategoryT2I},
		{"ja+en", "catのimageを作って", "ja-JP", CategoryT2I},
		{"de+en", "create ein bild", "de-DE", CategoryT2I},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyMediaIntent(tt.msg, false, 0, tt.locale)
			if got == nil {
				t.Fatalf("expected category=%s, got nil", tt.wantCat)
			}
			if got.Category != tt.wantCat {
				t.Errorf("category: got %s, want %s", got.Category, tt.wantCat)
			}
		})
	}
}

func TestClassifyMediaIntent_DensityAndMetaSuppression(t *testing.T) {
	tests := []struct {
		name      string
		msg       string
		hasImages bool
		imgCount  int
		locale    string
		wantNil   bool
		wantCat   MediaCategory
		minConf   float64
		maxConf   float64
	}{
		{
			name:    "zh meta discussion",
			msg:     "IR匹配关键词的时候，也需要考虑关键词命中的密度吧，比如在一大段文本内部出现了生成图片可能就不是这个意图",
			locale:  "zh-CN",
			wantNil: true,
		},
		{
			name:    "en meta discussion",
			msg:     "For the intent classifier, a long paragraph may mention generate images, but that should not trigger this intent because we're discussing keyword matching density.",
			locale:  "en-US",
			wantNil: true,
		},
		{
			name:    "zh low density long text",
			msg:     "这是一段很长的产品说明文本，主要在讨论文档整理、消息同步、权限控制和缓存策略。中间顺带提到系统未来也许会支持生成图片能力，但这里并不是在向你发出生成请求。",
			locale:  "zh-CN",
			wantNil: true,
		},
		{
			name:    "en long valid prompt",
			msg:     "Please generate a highly detailed image of a moonlit harbor with watercolor textures, warm reflections, and soft cinematic lighting.",
			locale:  "en-US",
			wantCat: CategoryT2I,
			minConf: 0.7,
		},
		{
			name:    "single meta cue still allows valid request",
			msg:     "Generate an image of a delivery route map in a clean infographic style.",
			locale:  "en-US",
			wantCat: CategoryT2I,
			minConf: 0.7,
		},
		{
			name:    "en long weak mention stays suppressed",
			msg:     "We should write onboarding docs, describe the workspace rules, and somewhere in the middle mention generate image support for future versions.",
			locale:  "en-US",
			wantNil: true,
		},
		{
			name:      "i2i remains confident with image context",
			msg:       "Please edit this image by replacing the background with a warm sunset beach and lightly retouching the colors.",
			hasImages: true,
			imgCount:  1,
			locale:    "en-US",
			wantCat:   CategoryI2I,
			minConf:   0.8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyMediaIntent(tt.msg, tt.hasImages, tt.imgCount, tt.locale)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("expected nil, got category=%s confidence=%.2f", got.Category, got.Confidence)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected category=%s, got nil", tt.wantCat)
			}
			if tt.wantCat != CategoryNone && got.Category != tt.wantCat {
				t.Fatalf("category: got %s, want %s", got.Category, tt.wantCat)
			}
			if tt.minConf > 0 && got.Confidence < tt.minConf {
				t.Fatalf("confidence: got %.2f, want >= %.2f", got.Confidence, tt.minConf)
			}
			if tt.maxConf > 0 && got.Confidence > tt.maxConf {
				t.Fatalf("confidence: got %.2f, want <= %.2f", got.Confidence, tt.maxConf)
			}
		})
	}
}

func TestLocaleToLangKey(t *testing.T) {
	tests := []struct {
		locale string
		want   string
	}{
		{"en-US", "en"},
		{"en-GB", "en"},
		{"zh-CN", "zh"},
		{"zh-TW", "zh"},
		{"ja-JP", "ja"},
		{"ko-KR", "ko"},
		{"de-DE", "de"},
		{"fr-FR", "fr"},
		{"es-ES", "es"},
		{"ca-ES", "es"}, // Catalan → Spanish
		{"ga-IE", "en"}, // Irish → English
		{"pt-BR", "pt"},
		{"ru-RU", "ru"},
		{"", "en"},      // empty → English
		{"xx-YY", "en"}, // unknown → English
	}

	for _, tt := range tests {
		t.Run(tt.locale, func(t *testing.T) {
			got := localeToLangKey(tt.locale)
			if got != tt.want {
				t.Errorf("localeToLangKey(%q) = %q, want %q", tt.locale, got, tt.want)
			}
		})
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
