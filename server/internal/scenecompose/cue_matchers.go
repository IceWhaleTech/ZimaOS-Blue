package scenecompose

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/textmatch"

type foldedCueMatcher struct {
	inner *textmatch.FoldedAhoMatcher
}

type cueChoice struct {
	value   string
	matcher *foldedCueMatcher
}

func newFoldedCueMatcher(cues []string) *foldedCueMatcher {
	return &foldedCueMatcher{inner: textmatch.NewFoldedAhoMatcher(cues)}
}

func (m *foldedCueMatcher) Contains(text string) bool {
	if m == nil || m.inner == nil {
		return false
	}
	return m.inner.ContainsAnyFold(text)
}

func (m *foldedCueMatcher) Count(text string) int {
	if m == nil || m.inner == nil {
		return 0
	}
	seen := make(map[int]struct{}, 8)
	m.inner.ScanFold(text, func(match textmatch.FoldedMatch) bool {
		seen[match.PatternIndex] = struct{}{}
		return false
	})
	return len(seen)
}

func matchCueChoice(text string, choices []cueChoice) string {
	for _, choice := range choices {
		if choice.matcher.Contains(text) {
			return choice.value
		}
	}
	return ""
}

func collectCueChoices(text string, choices []cueChoice) []string {
	values := make([]string, 0, len(choices))
	for _, choice := range choices {
		if choice.matcher.Contains(text) {
			values = append(values, choice.value)
		}
	}
	return values
}

var (
	directionalCueMatcher = newFoldedCueMatcher([]string{"left", "right", "左", "右"})

	backgroundInferenceChoices = []cueChoice{
		{value: "forest", matcher: newFoldedCueMatcher([]string{"forest", "woods", "树林", "森林"})},
		{value: "beach", matcher: newFoldedCueMatcher([]string{"beach", "sea", "ocean", "沙滩", "海边"})},
		{value: "cozy coffee shop", matcher: newFoldedCueMatcher([]string{"coffee shop", "cafe", "咖啡馆"})},
		{value: "city street", matcher: newFoldedCueMatcher([]string{"city", "street", "urban", "城市", "街道"})},
		{value: "indoor room", matcher: newFoldedCueMatcher([]string{"room", "bedroom", "living room", "房间", "客厅"})},
		{value: "mountain landscape", matcher: newFoldedCueMatcher([]string{"mountain", "hill", "山", "山谷"})},
		{value: "park", matcher: newFoldedCueMatcher([]string{"park", "garden", "公园", "花园"})},
	}

	objectTypeChoices = []cueChoice{
		{value: "cat", matcher: newFoldedCueMatcher([]string{"cat", "猫"})},
		{value: "dog", matcher: newFoldedCueMatcher([]string{"dog", "小狗", "狗"})},
		{value: "robot", matcher: newFoldedCueMatcher([]string{"robot", "机器人"})},
		{value: "bird", matcher: newFoldedCueMatcher([]string{"bird", "鸟"})},
		{value: "person", matcher: newFoldedCueMatcher([]string{"person", "woman", "man", "girl", "boy", "人物", "女孩", "男孩"})},
		{value: "car", matcher: newFoldedCueMatcher([]string{"car", "汽车", "车"})},
		{value: "house", matcher: newFoldedCueMatcher([]string{"house", "home", "房子"})},
	}

	attributeChoices = []cueChoice{
		{value: "orange", matcher: newFoldedCueMatcher([]string{"orange", "橘", "橙"})},
		{value: "black", matcher: newFoldedCueMatcher([]string{"black", "黑"})},
		{value: "white", matcher: newFoldedCueMatcher([]string{"white", "白"})},
		{value: "cute", matcher: newFoldedCueMatcher([]string{"cute", "adorable", "可爱"})},
		{value: "small", matcher: newFoldedCueMatcher([]string{"small", "tiny", "小"})},
		{value: "large", matcher: newFoldedCueMatcher([]string{"large", "big", "大"})},
		{value: "friendly", matcher: newFoldedCueMatcher([]string{"friendly", "smiling", "友好"})},
	}

	styleChoices = []cueChoice{
		{value: "cartoon", matcher: newFoldedCueMatcher([]string{"cartoon", "anime", "卡通"})},
		{value: "illustration", matcher: newFoldedCueMatcher([]string{"illustration", "插画"})},
	}

	lightingChoices = []cueChoice{
		{value: "warm sunset light", matcher: newFoldedCueMatcher([]string{"sunset", "golden hour"})},
		{value: "dramatic light", matcher: newFoldedCueMatcher([]string{"dramatic"})},
		{value: "soft ambient light", matcher: newFoldedCueMatcher([]string{"ambient"})},
	}

	timeOfDayChoices = []cueChoice{
		{value: "night", matcher: newFoldedCueMatcher([]string{"night", "晚上", "夜"})},
		{value: "sunset", matcher: newFoldedCueMatcher([]string{"sunset", "afternoon", "傍晚"})},
		{value: "morning", matcher: newFoldedCueMatcher([]string{"morning", "清晨"})},
	}

	weatherChoices = []cueChoice{
		{value: "rainy", matcher: newFoldedCueMatcher([]string{"rain", "雨"})},
		{value: "foggy", matcher: newFoldedCueMatcher([]string{"fog", "mist", "雾"})},
		{value: "snowy", matcher: newFoldedCueMatcher([]string{"snow", "雪"})},
	}

	cameraViewChoices = []cueChoice{
		{value: "top-down", matcher: newFoldedCueMatcher([]string{"top-down", "俯视"})},
		{value: "close-up", matcher: newFoldedCueMatcher([]string{"close-up", "特写"})},
	}

	layoutHorizontalChoices = []cueChoice{
		{value: "left", matcher: newFoldedCueMatcher([]string{"left", "左"})},
		{value: "right", matcher: newFoldedCueMatcher([]string{"right", "右"})},
	}

	layoutVerticalChoices = []cueChoice{
		{value: "high", matcher: newFoldedCueMatcher([]string{"high", "sky", "高"})},
		{value: "middle", matcher: newFoldedCueMatcher([]string{"middle", "中"})},
	}

	layoutScaleChoices = []cueChoice{
		{value: "small", matcher: newFoldedCueMatcher([]string{"small", "tiny", "小"})},
		{value: "large", matcher: newFoldedCueMatcher([]string{"large", "giant", "大"})},
	}

	layoutFloatingCueMatcher = newFoldedCueMatcher([]string{"flying", "floating", "飞"})

	backgroundSearchPrimaryCueMatcher   = newFoldedCueMatcher([]string{"background", "landscape"})
	backgroundSearchWallpaperCueMatcher = newFoldedCueMatcher([]string{"wallpaper"})
	backgroundSearchNegativeCueMatcher  = newFoldedCueMatcher([]string{"poster", "template", "shop", "buy", "logo", "icon"})

	foregroundSearchPositiveCueMatcher = newFoldedCueMatcher([]string{"png", "transparent", "isolated", "cutout"})
	foregroundSearchNegativeCueMatcher = newFoldedCueMatcher([]string{"vector", "clipart", "logo", "icon", "poster", "template"})
)

var objectTypeStopwords = map[string]struct{}{
	"a": {}, "an": {}, "the": {}, "with": {}, "in": {}, "on": {}, "at": {}, "and": {}, "of": {}, "to": {}, "soft": {},
}
