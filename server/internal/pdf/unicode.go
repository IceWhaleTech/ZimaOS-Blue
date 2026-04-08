package pdf

import (
	"strings"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/textmatch"
)

const malformedPDFTextRunes = "丌挃迓徆亍仸觃迖乊収幵冴挄叏枂迒彔丏诹吅迗迕枀劤劢叐劥巪顼癿迚旪夗觍乩巟杢戓仌揑栺拪実仹帯呾敁伕亝俇卋庩飢伔飠斱牉伒斲匚庪甴翿"

var malformedPDFTextReplacementPairs = []string{
	"丌", "不",
	"挃", "指",
	"迓", "还",
	"极", "构",
	"诧", "语",
	"徆", "很",
	"亍", "于",
	"仸", "任",
	"觃", "规",
	"迖", "远",
	"乊", "之",
	"収", "发",
	"轱", "轻",
	"吨", "含",
	"丼", "举",
	"幵", "并",
	"冴", "况",
	"巫", "已",
	"叧", "另",
	"挄", "按",
	"戒", "或",
	"轳", "较",
	"叏", "取",
	"枂", "析",
	"迒", "返",
	"返", "这",
	"诺", "读",
	"发", "变",
	"彔", "录",
	"丏", "且",
	"诹", "诺",
	"吅", "合",
	"尿", "局",
	"绅", "细",
	"轰", "轴",
	"迗", "违",
	"迕", "进",
	"秱", "移",
	"诨", "误",
	"恳", "患",
	"枀", "极",
	"劤", "努",
	"轲", "载",
	"叱", "史",
	"巩", "差",
	"劢", "动",
	"叐", "受",
	"囿", "圆",
	"史", "右",
	"劣", "助",
	"局", "层",
	"劥", "励",
	"巪", "己",
	"顼", "顾",
	"讣", "认",
	"癿", "的",
	"迚", "进",
	"旪", "时",
	"夗", "多",
	"觍", "计",
	"远", "违",
	"乩", "事",
	"不", "与",
	"绊", "绑",
	"卒", "卡",
	"栎", "样",
	"多", "大",
	"屍", "展",
	"巟", "工",
	"杢", "来",
	"戓", "战",
	"仌", "仍",
	"揑", "插",
	"兲", "关",
	"栺", "格",
	"拪", "括",
	"実", "审",
	"仹", "份",
	"帯", "常",
	"课", "调",
	"绉", "经",
	"叵", "司",
	"举", "么",
	"兰", "关",
	"抦", "批",
	"呾", "和",
	"敁", "效",
	"伕", "会",
	"细", "织",
	"亝", "交",
	"俇", "修",
	"卋", "协",
	"二", "于",
	"绚", "络",
	"申", "电",
	"光", "克",
	"庩", "康",
	"飢", "饮",
	"徇", "得",
	"伔", "伙",
	"甸", "界",
	"宠", "客",
	"艱", "色",
	"兇", "先",
	"徊", "德",
	"电", "甸",
	"匙", "区",
	"飠", "餐",
	"斱", "方",
	"逑", "递",
	"俅", "保",
	"弻", "归",
	"町", "略",
	"牉", "牌",
	"层", "屈",
	"织", "终",
	"与", "专",
	"翿", "考",
	"诠", "该",
	"事", "二",
	"巳", "巴",
	"违", "连",
	"伓", "优",
	"半", "华",
	"卍", "单",
	"俆", "信",
	"规", "视",
	"伒", "众",
	"斲", "施",
	"很", "循",
	"先", "光",
	"梱", "检",
	"匚", "医",
	"杆", "村",
	"埻", "堂",
	"审", "宣",
	"庪", "廉",
	"绛", "绝",
	"弽", "录",
	"邁", "那",
	"甴", "男",
	"贤", "败",
}

type malformedPDFTextFallback struct {
	matcherOnce  sync.Once
	replacerOnce sync.Once
	matcher      *textmatch.FoldedAhoMatcher
	replacer     *strings.Replacer
}

func (f *malformedPDFTextFallback) containsMalformedRunes(text string) bool {
	if f == nil || text == "" {
		return false
	}
	f.matcherOnce.Do(func() {
		patterns := make([]string, 0, len([]rune(malformedPDFTextRunes)))
		for _, r := range malformedPDFTextRunes {
			patterns = append(patterns, string(r))
		}
		f.matcher = textmatch.NewFoldedAhoMatcher(patterns)
	})
	return f.matcher != nil && f.matcher.ContainsAnyFold(text)
}

func (f *malformedPDFTextFallback) repair(text string) string {
	if f == nil || text == "" || !f.containsMalformedRunes(text) {
		return text
	}
	f.replacerOnce.Do(func() {
		f.replacer = strings.NewReplacer(malformedPDFTextReplacementPairs...)
	})
	if f.replacer == nil {
		return text
	}
	return f.replacer.Replace(text)
}

var pdfMalformedTextFallbackState malformedPDFTextFallback

func repairMalformedPDFText(text string) string {
	return pdfMalformedTextFallbackState.repair(text)
}
