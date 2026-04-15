package server

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

type pseudoXMLNode struct {
	Name     string
	Attrs    map[string]string
	Children []*pseudoXMLNode
	Text     strings.Builder
}

var pseudoXMLCompatAliasGroups = map[string][]string{
	"bash":        {"bash", "exec"},
	"file_delete": {"file_delete", "delete", "remove", "rm", "unlink"},
	"grep":        {"grep", "rg"},
	"image":       {"image", "image_generation", "generate_image", "generateimage"},
	"read":        {"read", "read_file", "file_read"},
	"research":    {"research", "deep_research", "deep-research", "research_run", "research_status"},
	"web_query":   {"web", "web_query", "web_search", "web_fetch", "web_read", "web_extract", "web_crawl"},
	"write":       {"write", "write_file", "file_write"},
}

var defaultPseudoXMLToolNames = []string{
	"analyze",
	"ask",
	"bash",
	"browser",
	"calendar",
	"contacts",
	"convert",
	"research",
	"deep_research",
	"edit",
	"email",
	"file_delete",
	"find",
	"grep",
	"image",
	"ls",
	"docx",
	"xlsx",
	"pptx",
	"pdf",
	"read",
	"reminder",
	"ui_reviewer",
	"web",
	"web_query",
	"web_search",
	"write",
}

func recoverPseudoToolCallsFromContent(content string, allowedTools []llm.Tool) ([]llm.ToolCall, bool) {
	ensureChatMiscRegexes()
	if len(allowedTools) == 0 {
		return nil, false
	}
	maskedContent := maskPseudoRecoveryExcludedRanges(content, allowedTools)
	var recovered []llm.ToolCall
	if jsonCalls, ok := recoverBareJSONPseudoToolCallsFromContent(maskedContent, allowedTools); ok {
		recovered = append(recovered, jsonCalls...)
	}
	if bracketedCalls, ok := recoverBracketedPseudoToolCallsFromContent(maskedContent, allowedTools); ok {
		recovered = append(recovered, bracketedCalls...)
	}
	nodes, err := parsePseudoXMLFragment(maskedContent)
	if err != nil {
		if len(recovered) > 0 {
			return recovered, true
		}
		return nil, false
	}
	for _, node := range nodes {
		collectRecoveredPseudoToolCalls(node, allowedTools, &recovered)
	}
	if len(recovered) == 0 {
		if jsonCalls, ok := recoverEmbeddedBareJSONPseudoToolCallsFromContent(content, allowedTools); ok {
			recovered = append(recovered, jsonCalls...)
		}
	}
	if len(recovered) == 0 {
		return nil, false
	}
	return recovered, true
}

func recoverBareJSONPseudoToolCallsFromContent(content string, allowedTools []llm.Tool) ([]llm.ToolCall, bool) {
	fragments, ok := extractTopLevelJSONFragments(content)
	if !ok {
		return nil, false
	}
	recovered := make([]llm.ToolCall, 0, len(fragments))
	for _, fragment := range fragments {
		calls, ok := recoverPseudoJSONToolCallsFragment(fragment, allowedTools)
		if !ok {
			return nil, false
		}
		recovered = append(recovered, calls...)
	}
	if len(recovered) == 0 {
		return nil, false
	}
	return recovered, true
}

func recoverBracketedPseudoToolCallsFromContent(content string, allowedTools []llm.Tool) ([]llm.ToolCall, bool) {
	matches := rePseudoBracketedToolCallBlock.FindAllString(content, -1)
	if len(matches) == 0 {
		return nil, false
	}
	recovered := make([]llm.ToolCall, 0, len(matches))
	for _, block := range matches {
		inner := strings.TrimSpace(rePseudoBracketedToolCallTag.ReplaceAllString(block, " "))
		if inner == "" {
			continue
		}
		if call, ok := recoverPseudoBracketedToolCall(inner, allowedTools); ok {
			recovered = append(recovered, call)
		}
	}
	if len(recovered) == 0 {
		return nil, false
	}
	return recovered, true
}

func pseudoJSONToolCallStartIndex(delta string, allowedTools []llm.Tool) int {
	matches := extractRecoverableEmbeddedPseudoJSONFragments(delta, allowedTools)
	if len(matches) == 0 {
		return -1
	}
	return matches[0].Start
}

func recoverSanitizedPseudoToolCallsFromContent(content string, allowedTools []llm.Tool) ([]llm.ToolCall, bool) {
	recovered, ok := recoverPseudoToolCallsFromContent(content, allowedTools)
	if !ok {
		return nil, false
	}
	sanitized, _ := sanitizeAssistantToolCallsForAllowedSet(recovered, allowedTools)
	if len(sanitized) == 0 {
		return nil, false
	}
	return sanitized, true
}

func pseudoXMLToolTagStartIndex(delta string, allowedTools []llm.Tool) int {
	lower := strings.ToLower(delta)
	minIndex := -1
	for _, name := range pseudoXMLToolTagCandidates(allowedTools) {
		pattern := "<" + strings.ToLower(name)
		idx := strings.Index(lower, pattern)
		for idx >= 0 {
			after := idx + len(pattern)
			if after < len(lower) {
				switch lower[after] {
				case '>', '/', ' ', '\n', '\r', '\t':
					if minIndex < 0 || idx < minIndex {
						minIndex = idx
					}
					idx = -1
					continue
				}
			}
			next := idx + 1
			found := strings.Index(lower[next:], pattern)
			if found < 0 {
				idx = -1
				continue
			}
			idx = next + found
		}
	}
	return minIndex
}

func looksLikeDirectXMLPseudoToolCall(s string) bool {
	nodes, err := parsePseudoXMLFragment(s)
	if err != nil {
		return false
	}
	for _, node := range nodes {
		if containsLikelyDirectXMLPseudoToolCall(node) {
			return true
		}
	}
	return false
}

func parsePseudoXMLFragment(content string) ([]*pseudoXMLNode, error) {
	decoder := xml.NewDecoder(strings.NewReader("<root>" + content + "</root>"))
	root := &pseudoXMLNode{Name: "root"}
	stack := []*pseudoXMLNode{root}

	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		switch tok := token.(type) {
		case xml.StartElement:
			node := &pseudoXMLNode{
				Name:  tok.Name.Local,
				Attrs: make(map[string]string, len(tok.Attr)),
			}
			for _, attr := range tok.Attr {
				node.Attrs[attr.Name.Local] = attr.Value
			}
			parent := stack[len(stack)-1]
			parent.Children = append(parent.Children, node)
			stack = append(stack, node)
		case xml.EndElement:
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			stack[len(stack)-1].Text.Write(tok)
		}
	}

	return root.Children, nil
}

func collectRecoveredPseudoToolCalls(node *pseudoXMLNode, allowedTools []llm.Tool, out *[]llm.ToolCall) {
	switch strings.ToLower(strings.TrimSpace(node.Name)) {
	case "function_calls":
		for _, child := range node.Children {
			collectRecoveredPseudoToolCalls(child, allowedTools, out)
		}
		return
	case "tool_call", "function_call":
		if call, ok := recoverPseudoToolCallWrapper(node, allowedTools); ok {
			*out = append(*out, call)
			return
		}
	case "tool_code":
		if call, ok := recoverPseudoToolCodeWrapper(node, allowedTools); ok {
			*out = append(*out, call)
			return
		}
	case "invoke":
		if call, ok := recoverPseudoInvokeToolCall(node, allowedTools); ok {
			*out = append(*out, call)
			return
		}
	}

	if call, ok := recoverDirectPseudoXMLToolCall(node, allowedTools); ok {
		*out = append(*out, call)
		return
	}

	for _, child := range node.Children {
		collectRecoveredPseudoToolCalls(child, allowedTools, out)
	}
}

func recoverPseudoInvokeToolCall(node *pseudoXMLNode, allowedTools []llm.Tool) (llm.ToolCall, bool) {
	if !strings.EqualFold(strings.TrimSpace(node.Name), "invoke") {
		return llm.ToolCall{}, false
	}
	invokeName := strings.TrimSpace(node.Attrs["name"])
	if invokeName == "" {
		return llm.ToolCall{}, false
	}

	paramValues := pseudoXMLNamedChildren(node)
	rawToolName := invokeName
	argsValue := any(map[string]interface{}{})

	if strings.EqualFold(invokeName, "$blue") {
		command, _ := paramValues["command"].(string)
		command = strings.TrimSpace(command)
		if command == "" {
			return llm.ToolCall{}, false
		}
		rawToolName = command
		if rawArgs, ok := paramValues["args"]; ok {
			argsValue = rawArgs
		} else {
			argsMap := make(map[string]interface{}, len(paramValues))
			for key, value := range paramValues {
				if strings.EqualFold(key, "command") {
					continue
				}
				argsMap[key] = value
			}
			if len(argsMap) == 0 {
				if scalar, ok := pseudoXMLNodeScalarValue(node); ok {
					argsValue = scalar
				} else {
					argsValue = argsMap
				}
			} else {
				argsValue = argsMap
			}
		}
	} else {
		if len(paramValues) == 0 {
			if scalar, ok := pseudoXMLNodeScalarValue(node); ok {
				argsValue = scalar
			} else {
				argsValue = paramValues
			}
		} else {
			argsValue = paramValues
		}
	}

	name, ok := resolveRecoveredPseudoToolName(rawToolName, allowedTools)
	if !ok {
		return llm.ToolCall{}, false
	}
	arguments, ok := marshalRecoveredPseudoToolArgs(name, argsValue, allowedTools)
	if !ok {
		return llm.ToolCall{}, false
	}
	return llm.ToolCall{Name: name, Arguments: arguments}, true
}

func recoverPseudoToolCallWrapper(node *pseudoXMLNode, allowedTools []llm.Tool) (llm.ToolCall, bool) {
	if node == nil {
		return llm.ToolCall{}, false
	}
	switch strings.ToLower(strings.TrimSpace(node.Name)) {
	case "tool_call", "function_call":
	default:
		return llm.ToolCall{}, false
	}

	rawToolName, argsValue, ok := extractPseudoToolCallWrapperPayload(node)
	if !ok {
		return llm.ToolCall{}, false
	}
	name, ok := resolveRecoveredPseudoToolName(rawToolName, allowedTools)
	if !ok {
		return llm.ToolCall{}, false
	}
	arguments, ok := marshalRecoveredPseudoToolArgs(name, argsValue, allowedTools)
	if !ok {
		return llm.ToolCall{}, false
	}
	return llm.ToolCall{Name: name, Arguments: arguments}, true
}

func recoverPseudoToolCodeWrapper(node *pseudoXMLNode, allowedTools []llm.Tool) (llm.ToolCall, bool) {
	if node == nil || !strings.EqualFold(strings.TrimSpace(node.Name), "tool_code") {
		return llm.ToolCall{}, false
	}

	scalar, ok := pseudoXMLNodeScalarValue(node)
	if !ok {
		return llm.ToolCall{}, false
	}
	body, ok := scalar.(string)
	if !ok {
		return llm.ToolCall{}, false
	}

	rawToolName, argsValue, ok := extractPseudoToolCodePayload(body)
	if !ok {
		return llm.ToolCall{}, false
	}
	name, ok := resolveRecoveredPseudoToolName(rawToolName, allowedTools)
	if !ok {
		return llm.ToolCall{}, false
	}
	arguments, ok := marshalRecoveredPseudoToolArgs(name, argsValue, allowedTools)
	if !ok {
		return llm.ToolCall{}, false
	}
	return llm.ToolCall{Name: name, Arguments: arguments}, true
}

func recoverPseudoBracketedToolCall(content string, allowedTools []llm.Tool) (llm.ToolCall, bool) {
	rawToolName, argsValue, ok := extractPseudoBracketedToolCallPayload(content)
	if !ok {
		return llm.ToolCall{}, false
	}
	name, ok := resolveRecoveredPseudoToolName(rawToolName, allowedTools)
	if !ok {
		return llm.ToolCall{}, false
	}
	arguments, ok := marshalRecoveredPseudoToolArgs(name, argsValue, allowedTools)
	if !ok {
		return llm.ToolCall{}, false
	}
	return llm.ToolCall{Name: name, Arguments: arguments}, true
}

func recoverPseudoJSONToolCallsFragment(content string, allowedTools []llm.Tool) ([]llm.ToolCall, bool) {
	decoded := coerceRecoveredPseudoToolScalar(content)
	return recoverPseudoJSONToolCallsValue(decoded, allowedTools)
}

type recoverablePseudoJSONFragment struct {
	Start int
	End   int
	Calls []llm.ToolCall
}

type pseudoContentRange struct {
	Start int
	End   int
}

func recoverEmbeddedBareJSONPseudoToolCallsFromContent(content string, allowedTools []llm.Tool) ([]llm.ToolCall, bool) {
	matches := extractRecoverableEmbeddedPseudoJSONFragments(content, allowedTools)
	if len(matches) == 0 {
		return nil, false
	}
	recovered := make([]llm.ToolCall, 0, len(matches))
	for _, match := range matches {
		recovered = append(recovered, match.Calls...)
	}
	if len(recovered) == 0 {
		return nil, false
	}
	return recovered, true
}

func extractRecoverableEmbeddedPseudoJSONFragments(content string, allowedTools []llm.Tool) []recoverablePseudoJSONFragment {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	protectedRanges := pseudoRecoveryMarkdownProtectedRanges(content)
	matches := make([]recoverablePseudoJSONFragment, 0, 1)
	for i := 0; i < len(content); i++ {
		switch content[i] {
		case '{', '[':
		default:
			continue
		}
		if pseudoIndexInRanges(i, protectedRanges) || pseudoRecoveryHasExampleContext(content, i) {
			continue
		}

		fragment, ok := extractBalancedJSONFragment(content[i:])
		if !ok {
			continue
		}
		end := i + len(fragment)
		if calls, ok := recoverPseudoJSONToolCallsFragment(fragment, allowedTools); ok && len(calls) > 0 {
			matches = append(matches, recoverablePseudoJSONFragment{
				Start: i,
				End:   end,
				Calls: calls,
			})
		}
		i = end - 1
	}
	if len(matches) == 0 {
		return nil
	}
	return matches
}

func pseudoRecoveryMarkdownProtectedRanges(content string) []pseudoContentRange {
	if content == "" {
		return nil
	}
	ranges := make([]pseudoContentRange, 0, 2)
	for i := 0; i < len(content); i++ {
		if !strings.HasPrefix(content[i:], "```") {
			continue
		}
		end := strings.Index(content[i+3:], "```")
		if end < 0 {
			ranges = append(ranges, pseudoContentRange{Start: i, End: len(content)})
			return ranges
		}
		end += i + 6
		ranges = append(ranges, pseudoContentRange{Start: i, End: end})
		i = end - 1
	}
	for i := 0; i < len(content); i++ {
		if content[i] != '`' || strings.HasPrefix(content[i:], "```") || pseudoIndexInRanges(i, ranges) {
			continue
		}
		end := strings.IndexByte(content[i+1:], '`')
		if end < 0 {
			break
		}
		end += i + 2
		ranges = append(ranges, pseudoContentRange{Start: i, End: end})
		i = end - 1
	}
	ranges = append(ranges, pseudoRecoveryQuotedOrListLineRanges(content)...)
	return ranges
}

func maskPseudoRecoveryExcludedRanges(content string, allowedTools []llm.Tool) string {
	ranges := pseudoRecoveryMarkdownProtectedRanges(content)
	ranges = append(ranges, pseudoRecoveryExampleSnippetRanges(content, allowedTools, ranges)...)
	return maskPseudoRecoveryRanges(content, ranges)
}

func maskPseudoRecoveryRanges(content string, ranges []pseudoContentRange) string {
	if len(ranges) == 0 {
		return content
	}
	buf := []byte(content)
	for _, r := range ranges {
		if r.Start < 0 {
			r.Start = 0
		}
		if r.End > len(buf) {
			r.End = len(buf)
		}
		for i := r.Start; i < r.End; i++ {
			if buf[i] == '\n' || buf[i] == '\r' {
				continue
			}
			buf[i] = ' '
		}
	}
	return string(buf)
}

func pseudoIndexInRanges(index int, ranges []pseudoContentRange) bool {
	for _, r := range ranges {
		if index >= r.Start && index < r.End {
			return true
		}
	}
	return false
}

func pseudoRecoveryExampleSnippetRanges(content string, allowedTools []llm.Tool, protectedRanges []pseudoContentRange) []pseudoContentRange {
	if content == "" {
		return nil
	}
	ranges := make([]pseudoContentRange, 0, 2)
	for i := 0; i < len(content); i++ {
		if pseudoIndexInRanges(i, protectedRanges) || pseudoIndexInRanges(i, ranges) || !pseudoRecoveryHasExampleContext(content, i) {
			continue
		}
		start, end, ok := pseudoRecoverySnippetRangeAt(content, i, allowedTools)
		if !ok {
			continue
		}
		ranges = append(ranges, pseudoContentRange{Start: start, End: end})
		i = end - 1
	}
	return ranges
}

func pseudoRecoverySnippetRangeAt(content string, start int, allowedTools []llm.Tool) (int, int, bool) {
	if start < 0 || start >= len(content) {
		return 0, 0, false
	}
	switch content[start] {
	case '[':
		if loc := rePseudoBracketedToolCallBlock.FindStringIndex(content[start:]); len(loc) == 2 && loc[0] == 0 {
			return start, start + loc[1], true
		}
		if loc := rePseudoBracketedToolCallTag.FindStringIndex(content[start:]); len(loc) == 2 && loc[0] == 0 {
			return start, start + loc[1], true
		}
		fragment, ok := extractBalancedJSONFragment(content[start:])
		if !ok {
			return 0, 0, false
		}
		return start, start + len(fragment), true
	case '{':
		fragment, ok := extractBalancedJSONFragment(content[start:])
		if !ok {
			return 0, 0, false
		}
		return start, start + len(fragment), true
	case '<':
		return pseudoRecoveryXMLSnippetRangeAt(content, start, allowedTools)
	default:
		return 0, 0, false
	}
}

func pseudoRecoveryXMLSnippetRangeAt(content string, start int, allowedTools []llm.Tool) (int, int, bool) {
	if start < 0 || start >= len(content) || content[start] != '<' {
		return 0, 0, false
	}
	tagEndRel := strings.IndexByte(content[start:], '>')
	if tagEndRel < 0 {
		return 0, 0, false
	}
	tagEnd := start + tagEndRel + 1
	name := pseudoXMLStartTagName(content[start:tagEnd])
	if !isPotentialPseudoXMLRecoveryTagName(name, allowedTools) {
		return 0, 0, false
	}
	if tagEnd-start >= 2 && content[tagEnd-2] == '/' {
		return start, tagEnd, true
	}
	closingTag := "</" + strings.ToLower(name) + ">"
	lower := strings.ToLower(content[tagEnd:])
	closeRel := strings.Index(lower, closingTag)
	if closeRel < 0 {
		return start, tagEnd, true
	}
	return start, tagEnd + closeRel + len(closingTag), true
}

func pseudoXMLStartTagName(tag string) string {
	tag = strings.TrimSpace(tag)
	if len(tag) < 3 || tag[0] != '<' || tag[1] == '/' {
		return ""
	}
	i := 1
	for i < len(tag) {
		ch := tag[i]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-' || ch == '.' || ch == ':' {
			i++
			continue
		}
		break
	}
	if i <= 1 {
		return ""
	}
	return tag[1:i]
}

func isPotentialPseudoXMLRecoveryTagName(name string, allowedTools []llm.Tool) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" {
		return false
	}
	switch lower {
	case "function_calls", "tool_call", "function_call", "tool_code", "invoke", "parameter":
		return true
	}
	for _, candidate := range pseudoXMLToolTagCandidates(allowedTools) {
		if lower == strings.ToLower(candidate) {
			return true
		}
	}
	return false
}

func pseudoRecoveryHasExampleContext(content string, start int) bool {
	if start < 0 || start > len(content) {
		return false
	}
	windowStart := start - 64
	if windowStart < 0 {
		windowStart = 0
	}
	before := strings.ToLower(content[windowStart:start])
	for _, marker := range []string{
		"json example",
		"tool example",
		"tool-call example",
		"tool call example",
		"format below",
		"format as",
		"return the following",
		"output the following",
		"you can return",
		"you can output",
		"write it as",
		"for example",
		"example:",
		"example：",
		"documentation:",
		"documentation note",
		"doc note",
		"response format",
		"example output",
		"示例",
		"例如",
		"比如",
		"样例",
		"参考这个",
		"文档说明",
		"说明：",
		"可选写法",
		"写法如下",
		"如下写法",
		"以下写法",
		"响应格式",
		"格式如下",
		"按如下格式",
		"返回以下",
		"输出以下",
		"可以返回以下",
		"可以输出以下",
		"你可以返回以下",
		"你可以输出以下",
		"可写成",
		"可以写成",
		"写成如下",
		"调用示例",
		"工具调用示例",
	} {
		if strings.Contains(before, marker) {
			return true
		}
	}
	return false
}

func pseudoRecoveryQuotedOrListLineRanges(content string) []pseudoContentRange {
	if content == "" {
		return nil
	}
	ranges := make([]pseudoContentRange, 0, 2)
	lineStart := 0
	for lineStart < len(content) {
		lineEnd := strings.IndexByte(content[lineStart:], '\n')
		if lineEnd < 0 {
			lineEnd = len(content)
		} else {
			lineEnd += lineStart
		}
		line := content[lineStart:lineEnd]
		prefixLen, ok := pseudoRecoveryQuotedOrListPrefixLen(line)
		if ok {
			candidate := strings.TrimSpace(line[prefixLen:])
			if pseudoLineContainsLikelyPseudoSnippet(candidate) {
				ranges = append(ranges, pseudoContentRange{Start: lineStart, End: lineEnd})
			}
		}
		if lineEnd == len(content) {
			break
		}
		lineStart = lineEnd + 1
	}
	return ranges
}

func pseudoRecoveryQuotedOrListPrefixLen(line string) (int, bool) {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	if i >= len(line) {
		return 0, false
	}
	if line[i] == '>' {
		for i < len(line) && line[i] == '>' {
			i++
		}
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		return i, true
	}
	if strings.HasPrefix(line[i:], "- ") || strings.HasPrefix(line[i:], "* ") || strings.HasPrefix(line[i:], "+ ") {
		return i + 2, true
	}
	j := i
	for j < len(line) && line[j] >= '0' && line[j] <= '9' {
		j++
	}
	if j > i && j+1 < len(line) && line[j] == '.' && line[j+1] == ' ' {
		return j + 2, true
	}
	return 0, false
}

func pseudoLineContainsLikelyPseudoSnippet(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "[tool_call]") ||
		strings.Contains(lower, "[/tool_call]") ||
		strings.Contains(lower, "[tool_use]") ||
		strings.Contains(lower, "[/tool_use]") ||
		strings.Contains(lower, `<function_calls>`) ||
		strings.Contains(lower, `<tool_call`) ||
		strings.Contains(lower, `<function_call`) ||
		strings.Contains(lower, `<invoke `) ||
		strings.Contains(lower, `<parameter `) ||
		strings.Contains(lower, `{tool =>`) ||
		strings.Contains(lower, "args =>") {
		return true
	}
	if strings.Contains(lower, `"arguments"`) &&
		(strings.Contains(lower, `"name"`) || strings.Contains(lower, `"tool_calls"`) || strings.Contains(lower, `"function"`)) {
		return true
	}
	return looksLikeDirectXMLPseudoToolCall(trimmed)
}

func recoverPseudoJSONToolCall(content string, allowedTools []llm.Tool) (llm.ToolCall, bool) {
	typed, ok := coerceRecoveredPseudoToolScalar(content).(map[string]interface{})
	if !ok {
		return llm.ToolCall{}, false
	}
	rawToolName, argsValue, ok := extractPseudoToolCallWrapperPayloadFromMap(typed)
	if !ok {
		return llm.ToolCall{}, false
	}
	name, ok := resolveRecoveredPseudoToolName(rawToolName, allowedTools)
	if !ok {
		return llm.ToolCall{}, false
	}
	arguments, ok := marshalRecoveredPseudoToolArgs(name, argsValue, allowedTools)
	if !ok {
		return llm.ToolCall{}, false
	}
	return llm.ToolCall{Name: name, Arguments: arguments}, true
}

func recoverPseudoJSONToolCallsValue(value interface{}, allowedTools []llm.Tool) ([]llm.ToolCall, bool) {
	switch typed := value.(type) {
	case map[string]interface{}:
		if rawToolName, argsValue, ok := extractPseudoToolCallWrapperPayloadFromMap(typed); ok {
			name, ok := resolveRecoveredPseudoToolName(rawToolName, allowedTools)
			if !ok {
				return nil, false
			}
			arguments, ok := marshalRecoveredPseudoToolArgs(name, argsValue, allowedTools)
			if !ok {
				return nil, false
			}
			return []llm.ToolCall{{Name: name, Arguments: arguments}}, true
		}

		var recovered []llm.ToolCall
		for _, key := range []string{"tool_calls", "toolcalls", "choices", "choice", "delta", "message", "response", "data"} {
			child, ok := typed[key]
			if !ok {
				continue
			}
			calls, ok := recoverPseudoJSONToolCallsValue(child, allowedTools)
			if !ok {
				continue
			}
			recovered = append(recovered, calls...)
		}
		if len(recovered) == 0 {
			return nil, false
		}
		return recovered, true

	case []interface{}:
		var recovered []llm.ToolCall
		for _, item := range typed {
			calls, ok := recoverPseudoJSONToolCallsValue(item, allowedTools)
			if !ok {
				continue
			}
			recovered = append(recovered, calls...)
		}
		if len(recovered) == 0 {
			return nil, false
		}
		return recovered, true
	default:
		return nil, false
	}
}

func recoverDirectPseudoXMLToolCall(node *pseudoXMLNode, allowedTools []llm.Tool) (llm.ToolCall, bool) {
	if node == nil {
		return llm.ToolCall{}, false
	}
	rawToolName := strings.TrimSpace(node.Name)
	if rawToolName == "" {
		return llm.ToolCall{}, false
	}
	name, ok := resolveRecoveredPseudoToolName(rawToolName, allowedTools)
	if !ok {
		return llm.ToolCall{}, false
	}
	var argsValue interface{}
	if len(node.Children) > 0 {
		argsValue = pseudoXMLNamedChildren(node)
	} else if scalar, ok := pseudoXMLNodeScalarValue(node); ok {
		argsValue = scalar
	} else {
		return llm.ToolCall{}, false
	}
	arguments, ok := marshalRecoveredPseudoToolArgs(name, argsValue, allowedTools)
	if !ok {
		return llm.ToolCall{}, false
	}
	return llm.ToolCall{Name: name, Arguments: arguments}, true
}

func extractPseudoToolCallWrapperPayload(node *pseudoXMLNode) (string, interface{}, bool) {
	if node == nil {
		return "", nil, false
	}
	if rawToolName, argsValue, ok := extractPseudoToolCallWrapperPayloadFromMap(pseudoXMLNamedChildren(node)); ok {
		return rawToolName, argsValue, true
	}
	scalar, ok := pseudoXMLNodeScalarValue(node)
	if !ok {
		return "", nil, false
	}
	typed, ok := scalar.(map[string]interface{})
	if !ok {
		return "", nil, false
	}
	return extractPseudoToolCallWrapperPayloadFromMap(typed)
}

func extractPseudoBracketedToolCallPayload(content string) (string, interface{}, bool) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", nil, false
	}
	if typed, ok := coerceRecoveredPseudoToolScalar(content).(map[string]interface{}); ok {
		if rawToolName, argsValue, ok := extractPseudoToolCallWrapperPayloadFromMap(typed); ok {
			return rawToolName, argsValue, true
		}
	}

	rawToolName := extractBracketedPseudoToolName(content)
	if rawToolName == "" {
		return "", nil, false
	}
	if argsValue, ok := extractBracketedPseudoArgsValue(content); ok {
		return rawToolName, argsValue, true
	}
	return rawToolName, map[string]interface{}{}, true
}

func extractPseudoToolCallWrapperPayloadFromMap(values map[string]interface{}) (string, interface{}, bool) {
	if len(values) == 0 {
		return "", nil, false
	}
	for _, key := range []string{"function", "tool", "call"} {
		nested, ok := values[key]
		if !ok {
			continue
		}
		nestedMap, ok := nested.(map[string]interface{})
		if !ok {
			continue
		}
		if rawToolName, argsValue, ok := extractPseudoToolCallWrapperPayloadFromMap(nestedMap); ok {
			return rawToolName, argsValue, true
		}
	}
	rawToolName := firstPseudoToolCallWrapperString(values,
		"name",
		"tool",
		"function",
		"tool_name",
		"toolname",
		"function_name",
		"functionname",
	)
	if rawToolName == "" {
		return "", nil, false
	}
	if rawArgs, ok := firstPseudoToolCallWrapperValue(values, "arguments", "args", "parameters", "parameter"); ok {
		return rawToolName, normalizeRecoveredPseudoWrapperArgsValue(rawArgs), true
	}

	argsMap := make(map[string]interface{}, len(values))
	for key, value := range values {
		if isPseudoToolCallWrapperMetaKey(key) {
			continue
		}
		argsMap[key] = normalizeRecoveredPseudoWrapperArgsValue(value)
	}
	if len(argsMap) == 0 {
		return rawToolName, map[string]interface{}{}, true
	}
	return rawToolName, argsMap, true
}

func extractPseudoToolCodePayload(content string) (string, interface{}, bool) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "", nil, false
	}

	nameEnd := 0
	for nameEnd < len(trimmed) && !isPseudoToolCodeWhitespace(trimmed[nameEnd]) {
		nameEnd++
	}
	rawToolName := strings.TrimSpace(trimmed[:nameEnd])
	if rawToolName == "" {
		return "", nil, false
	}

	rest := strings.TrimSpace(trimmed[nameEnd:])
	if rest == "" {
		return rawToolName, map[string]interface{}{}, true
	}

	args := make(map[string]interface{})
	for len(rest) > 0 {
		key, value, remaining, ok := consumePseudoToolCodeAssignment(rest)
		if !ok {
			return "", nil, false
		}
		appendPseudoXMLMapValue(args, key, value)
		rest = strings.TrimSpace(remaining)
	}
	return rawToolName, args, true
}

func consumePseudoToolCodeAssignment(input string) (string, interface{}, string, bool) {
	s := strings.TrimLeft(input, " \t\r\n")
	if s == "" {
		return "", nil, "", false
	}

	keyEnd := 0
	for keyEnd < len(s) && isPseudoToolCodeKeyChar(s[keyEnd]) {
		keyEnd++
	}
	if keyEnd == 0 {
		return "", nil, "", false
	}
	key := strings.ReplaceAll(strings.TrimSpace(s[:keyEnd]), "-", "_")
	s = strings.TrimLeft(s[keyEnd:], " \t\r\n")
	if key == "" || s == "" || s[0] != '=' {
		return "", nil, "", false
	}

	s = strings.TrimLeft(s[1:], " \t\r\n")
	if s == "" {
		return "", nil, "", false
	}

	var (
		rawValue  string
		remaining string
		ok        bool
	)
	switch s[0] {
	case '"':
		rawValue, remaining, ok = consumePseudoToolCodeQuotedValue(s, '"')
	case '\'':
		rawValue, remaining, ok = consumePseudoToolCodeQuotedValue(s, '\'')
	default:
		valueEnd := 0
		for valueEnd < len(s) && !isPseudoToolCodeWhitespace(s[valueEnd]) {
			valueEnd++
		}
		rawValue = s[:valueEnd]
		remaining = s[valueEnd:]
		ok = strings.TrimSpace(rawValue) != ""
	}
	if !ok {
		return "", nil, "", false
	}

	return key, coerceRecoveredPseudoToolScalar(rawValue), remaining, true
}

func consumePseudoToolCodeQuotedValue(input string, quote byte) (string, string, bool) {
	if input == "" || input[0] != quote {
		return "", "", false
	}
	escaped := false
	for i := 1; i < len(input); i++ {
		ch := input[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch != quote {
			continue
		}

		token := input[:i+1]
		remaining := input[i+1:]
		if quote == '"' {
			if unquoted, err := strconv.Unquote(token); err == nil {
				return unquoted, remaining, true
			}
		}
		return token[1:i], remaining, true
	}
	return "", "", false
}

func isPseudoToolCodeWhitespace(ch byte) bool {
	switch ch {
	case ' ', '\t', '\r', '\n':
		return true
	default:
		return false
	}
}

func isPseudoToolCodeKeyChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') ||
		(ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9') ||
		ch == '_' ||
		ch == '-' ||
		ch == '.'
}

func firstPseudoToolCallWrapperString(values map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func firstPseudoToolCallWrapperValue(values map[string]interface{}, keys ...string) (interface{}, bool) {
	for _, key := range keys {
		value, ok := values[key]
		if ok {
			return value, true
		}
	}
	return nil, false
}

func isPseudoToolCallWrapperMetaKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "name", "tool", "function", "tool_name", "toolname", "function_name", "functionname", "id", "index", "type":
		return true
	default:
		return false
	}
}

func normalizeRecoveredPseudoWrapperArgsValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case string:
		return coerceRecoveredPseudoToolScalar(typed)
	case []interface{}:
		out := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			out = append(out, normalizeRecoveredPseudoWrapperArgsValue(item))
		}
		return out
	case map[string]interface{}:
		out := make(map[string]interface{}, len(typed))
		for key, item := range typed {
			out[key] = normalizeRecoveredPseudoWrapperArgsValue(item)
		}
		return out
	default:
		return value
	}
}

var reBracketedPseudoToolName = regexp.MustCompile(`(?is)\b(?:tool|name|function)\s*=>\s*("([^"]*)"|'([^']*)')`)

func extractBracketedPseudoToolName(content string) string {
	match := reBracketedPseudoToolName.FindStringSubmatch(content)
	if len(match) >= 4 {
		if strings.TrimSpace(match[2]) != "" {
			return strings.TrimSpace(match[2])
		}
		if strings.TrimSpace(match[3]) != "" {
			return strings.TrimSpace(match[3])
		}
	}
	return ""
}

func extractBracketedPseudoArgsValue(content string) (interface{}, bool) {
	lower := strings.ToLower(content)
	for _, key := range []string{"arguments", "args", "parameters", "parameter"} {
		idx := strings.Index(lower, key)
		if idx < 0 {
			continue
		}
		after := strings.TrimSpace(content[idx+len(key):])
		if !strings.HasPrefix(after, "=>") {
			continue
		}
		after = strings.TrimSpace(after[2:])
		if after == "" {
			continue
		}
		if after[0] == '{' {
			if block, ok := extractBalancedDelimitedBlock(after, '{', '}'); ok {
				if parsed, ok := parseBracketedPseudoArgsBlock(block[1 : len(block)-1]); ok {
					return parsed, true
				}
				return normalizeRecoveredPseudoWrapperArgsValue(block), true
			}
		}
		return normalizeRecoveredPseudoWrapperArgsValue(after), true
	}
	return nil, false
}

func extractBalancedDelimitedBlock(s string, open, close byte) (string, bool) {
	if len(s) == 0 || s[0] != open {
		return "", false
	}
	depth := 0
	inSingle := false
	inDouble := false
	escaped := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}
		if inSingle || inDouble {
			continue
		}
		switch ch {
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return s[:i+1], true
			}
		}
	}
	return "", false
}

func parseBracketedPseudoArgsBlock(content string) (map[string]interface{}, bool) {
	content = strings.TrimSpace(content)
	if content == "" {
		return map[string]interface{}{}, true
	}
	if typed, ok := coerceRecoveredPseudoToolScalar(content).(map[string]interface{}); ok {
		return typed, true
	}
	args := make(map[string]interface{})
	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		key, value, ok := parseBracketedPseudoFlagLine(line)
		if !ok {
			continue
		}
		appendPseudoXMLMapValue(args, key, value)
	}
	if len(args) == 0 {
		return nil, false
	}
	return args, true
}

func parseBracketedPseudoFlagLine(line string) (string, interface{}, bool) {
	line = strings.TrimSpace(strings.TrimRight(line, ","))
	if !strings.HasPrefix(line, "--") {
		return "", nil, false
	}
	line = strings.TrimSpace(line[2:])
	if line == "" {
		return "", nil, false
	}
	keyEnd := strings.IndexAny(line, " \t")
	if keyEnd <= 0 {
		return "", nil, false
	}
	key := strings.ReplaceAll(strings.TrimSpace(line[:keyEnd]), "-", "_")
	valueText := strings.TrimSpace(line[keyEnd+1:])
	if key == "" || valueText == "" {
		return "", nil, false
	}
	if len(valueText) >= 2 && valueText[0] == '"' && valueText[len(valueText)-1] == '"' {
		if unquoted, err := strconv.Unquote(valueText); err == nil {
			return key, coerceRecoveredPseudoToolScalar(unquoted), true
		}
	}
	if len(valueText) >= 2 && valueText[0] == '\'' && valueText[len(valueText)-1] == '\'' {
		return key, coerceRecoveredPseudoToolScalar(valueText[1 : len(valueText)-1]), true
	}
	if fields := strings.Fields(valueText); len(fields) > 0 {
		valueText = fields[0]
	}
	return key, coerceRecoveredPseudoToolScalar(valueText), true
}

func extractTopLevelJSONFragments(content string) ([]string, bool) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" || (trimmed[0] != '{' && trimmed[0] != '[') {
		return nil, false
	}
	fragments := make([]string, 0, 1)
	for len(trimmed) > 0 {
		fragment, ok := extractBalancedJSONFragment(trimmed)
		if !ok {
			return nil, false
		}
		fragments = append(fragments, fragment)
		trimmed = strings.TrimSpace(trimmed[len(fragment):])
		if trimmed == "" {
			break
		}
		if trimmed[0] != '{' && trimmed[0] != '[' {
			return nil, false
		}
	}
	if len(fragments) == 0 {
		return nil, false
	}
	return fragments, true
}

func extractBalancedJSONFragment(s string) (string, bool) {
	if s == "" {
		return "", false
	}
	switch s[0] {
	case '{':
		return extractBalancedDelimitedBlock(s, '{', '}')
	case '[':
		return extractBalancedDelimitedBlock(s, '[', ']')
	default:
		return "", false
	}
}

func resolveRecoveredPseudoToolName(raw string, allowedTools []llm.Tool) (string, bool) {
	candidates := recoveredPseudoToolNameCandidates(raw, allowedTools)
	for _, candidate := range candidates {
		normalized := strings.TrimSpace(normalizeAssistantToolCallNameForAllowedSet(candidate, allowedTools))
		if normalized == "" {
			continue
		}
		if containsLLMToolName(allowedTools, normalized) {
			return normalized, true
		}
	}
	return "", false
}

func recoveredPseudoToolNameCandidates(raw string, allowedTools []llm.Tool) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	seen := map[string]struct{}{trimmed: {}}
	out := []string{trimmed}
	lower := strings.ToLower(trimmed)

	uniqueMatch := ""
	for _, tool := range allowedTools {
		name := strings.TrimSpace(tool.Name)
		if name == "" {
			continue
		}
		for _, alias := range recoveredPseudoToolNameAliases(name) {
			if !matchesRecoveredPseudoToolAliasSuffix(lower, alias) {
				continue
			}
			if uniqueMatch != "" && uniqueMatch != name {
				return out
			}
			uniqueMatch = name
			break
		}
	}
	if uniqueMatch != "" {
		if _, ok := seen[uniqueMatch]; !ok {
			out = append(out, uniqueMatch)
		}
	}
	return out
}

func recoveredPseudoToolNameAliases(name string) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	out := []string{name}
	if aliases, ok := pseudoXMLCompatAliasGroups[normalizeFileToolCompatName(name)]; ok {
		out = append(out, aliases...)
	}
	return out
}

func matchesRecoveredPseudoToolAliasSuffix(rawLower, alias string) bool {
	alias = strings.ToLower(strings.TrimSpace(alias))
	if rawLower == "" || alias == "" {
		return false
	}
	if rawLower == alias {
		return true
	}
	if !strings.HasSuffix(rawLower, alias) || len(rawLower) <= len(alias) {
		return false
	}
	prev := rawLower[len(rawLower)-len(alias)-1]
	switch prev {
	case '_', '-', '.', ':', '/':
		return true
	default:
		return false
	}
}

func marshalRecoveredPseudoToolArgs(toolName string, argsValue interface{}, allowedTools []llm.Tool) (string, bool) {
	switch typed := argsValue.(type) {
	case nil:
		return "{}", true
	case map[string]interface{}:
		if typed == nil {
			return "{}", true
		}
	default:
		argsValue = wrapScalarRecoveredPseudoToolArg(toolName, argsValue, allowedTools)
	}

	b, err := json.Marshal(argsValue)
	if err != nil {
		return "", false
	}
	return string(b), true
}

func wrapScalarRecoveredPseudoToolArg(toolName string, value interface{}, allowedTools []llm.Tool) map[string]interface{} {
	if strings.TrimSpace(toolName) == "" {
		return map[string]interface{}{"input": value}
	}
	for _, tool := range allowedTools {
		if strings.TrimSpace(tool.Name) != toolName {
			continue
		}
		if props, ok := tool.Parameters["properties"].(map[string]interface{}); ok && len(props) == 1 {
			for key := range props {
				return map[string]interface{}{key: value}
			}
		}
		break
	}

	switch normalizeFileToolCompatName(toolName) {
	case "research":
		return map[string]interface{}{"query": value}
	default:
		return map[string]interface{}{"input": value}
	}
}

func pseudoXMLNamedChildren(node *pseudoXMLNode) map[string]interface{} {
	out := make(map[string]interface{}, len(node.Children))
	for _, child := range node.Children {
		key := strings.TrimSpace(child.Name)
		if strings.EqualFold(key, "parameter") {
			key = strings.TrimSpace(child.Attrs["name"])
		}
		if key == "" {
			continue
		}
		appendPseudoXMLMapValue(out, key, pseudoXMLNodeValue(child))
	}
	return out
}

func appendPseudoXMLMapValue(dst map[string]interface{}, key string, value interface{}) {
	if existing, ok := dst[key]; ok {
		if list, ok := existing.([]interface{}); ok {
			dst[key] = append(list, value)
			return
		}
		dst[key] = []interface{}{existing, value}
		return
	}
	dst[key] = value
}

func pseudoXMLNodeValue(node *pseudoXMLNode) interface{} {
	if node == nil {
		return ""
	}
	if len(node.Children) == 0 {
		return coerceRecoveredPseudoToolScalar(node.Text.String())
	}
	values := pseudoXMLNamedChildren(node)
	if len(values) > 0 {
		return values
	}
	return coerceRecoveredPseudoToolScalar(node.Text.String())
}

func pseudoXMLNodeScalarValue(node *pseudoXMLNode) (interface{}, bool) {
	if node == nil {
		return nil, false
	}
	trimmed := strings.TrimSpace(node.Text.String())
	if trimmed == "" {
		return nil, false
	}
	return coerceRecoveredPseudoToolScalar(trimmed), true
}

func coerceRecoveredPseudoToolScalar(raw string) interface{} {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		var decoded interface{}
		if json.Unmarshal([]byte(trimmed), &decoded) == nil {
			return decoded
		}
	}
	if v, err := strconv.ParseBool(strings.ToLower(trimmed)); err == nil {
		return v
	}
	if !strings.ContainsAny(trimmed, ".eE") {
		if v, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
			return v
		}
	}
	if strings.ContainsAny(trimmed, ".eE") {
		if v, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return v
		}
	}
	return trimmed
}

func containsLikelyDirectXMLPseudoToolCall(node *pseudoXMLNode) bool {
	if node == nil {
		return false
	}
	if isLikelyPseudoXMLToolName(node.Name) {
		if len(node.Children) > 0 {
			return true
		}
		if _, ok := pseudoXMLNodeScalarValue(node); ok {
			return true
		}
	}
	for _, child := range node.Children {
		if containsLikelyDirectXMLPseudoToolCall(child) {
			return true
		}
	}
	return false
}

func isLikelyPseudoXMLToolName(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" {
		return false
	}
	for _, candidate := range pseudoXMLToolTagCandidates(nil) {
		if lower == strings.ToLower(candidate) {
			return true
		}
	}
	return false
}

func pseudoXMLToolTagCandidates(allowedTools []llm.Tool) []string {
	set := make(map[string]struct{})
	add := func(name string) {
		trimmed := strings.TrimSpace(strings.ToLower(name))
		if trimmed == "" {
			return
		}
		set[trimmed] = struct{}{}
	}

	if len(allowedTools) == 0 {
		for _, name := range defaultPseudoXMLToolNames {
			add(name)
		}
		for _, aliases := range pseudoXMLCompatAliasGroups {
			for _, alias := range aliases {
				add(alias)
			}
		}
	} else {
		for _, tool := range allowedTools {
			name := strings.TrimSpace(tool.Name)
			if name == "" {
				continue
			}
			add(name)
			if aliases, ok := pseudoXMLCompatAliasGroups[normalizeFileToolCompatName(name)]; ok {
				for _, alias := range aliases {
					add(alias)
				}
			}
		}
	}

	add("tool_code")

	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	return out
}
