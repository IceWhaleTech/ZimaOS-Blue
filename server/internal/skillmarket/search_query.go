package skillmarket

import (
	"regexp"
	"strings"
)

const skillDiscoveryRequestZH = `(?:(?:请|麻烦你?|劳驾)\s*)?(?:(?:帮我|为我|给我|替我|我想要?)\s*)?(?:找(?:一找)?|查找|搜索|搜|推荐|有没有)(?:一下)?\s*(?:一个|一款|一种|几个|一些|个)?\s*`

var skillDiscoveryQueries = []*regexp.Regexp{
	regexp.MustCompile(`(?s)^` + skillDiscoveryRequestZH + `(?:技能|skills?)(?:\s*[，,:：]\s*|\s+)(.+)$`),
	regexp.MustCompile(`(?s)^` + skillDiscoveryRequestZH + `(.+?)的技能\s*[。！？!?]?$`),
	regexp.MustCompile(`(?is)^(?:(?:please,?|can you|could you|would you)\s+)*(?:help me\s+)?(?:find|search for|look for|recommend|suggest)(?:\s+me)?\s+(?:(?:a|an|some)\s+)?skills?\b(?:\s*[:,]\s*|\s+(?:(?:for|to|that can|which can)\s+)?)(.+)$`),
}

// RewriteSearchQuery removes explicit skill-discovery wrappers, preserving the
// capability text, including search verbs, identifiers, quotes and constraints.
// Unknown or underspecified phrasing keeps the original query.
func RewriteSearchQuery(query string) string {
	query = strings.TrimSpace(query)
	for _, pattern := range skillDiscoveryQueries {
		if match := pattern.FindStringSubmatch(query); match != nil {
			capability := strings.TrimSpace(match[1])
			if strings.Trim(capability, "，,:：") == "" {
				return query
			}
			switch {
			case strings.EqualFold(capability, "for"), strings.EqualFold(capability, "to"),
				strings.EqualFold(capability, "that can"), strings.EqualFold(capability, "which can"):
				return query
			default:
				return capability
			}
		}
	}
	return query
}
