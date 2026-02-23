package humanizer

import (
	"sort"
	"strings"
)

// StyleMarker defines the open/close markers for a style.
type StyleMarker struct {
	Open  string
	Close string
}

// LinkRenderer builds open/close strings for a link.
// Returns ok=false to skip link rendering.
type LinkRenderer func(link LinkSpan, fullText string) (open, close string, ok bool)

// RenderOptions controls how an IR is rendered to a target format.
type RenderOptions struct {
	Styles     map[Style]StyleMarker
	EscapeText func(string) string
	BuildLink  LinkRenderer
}

// Render converts an IR to a formatted string using the given options.
// Uses a boundary-based algorithm: collect all style/link start/end positions,
// sort them, and emit text segments with appropriate open/close markers.
func Render(ir IR, opts RenderOptions) string {
	text := ir.Text
	if text == "" {
		return ""
	}

	escape := opts.EscapeText
	if escape == nil {
		escape = func(s string) string { return s }
	}

	// Collect boundary points
	boundaries := make(map[int]struct{})
	boundaries[0] = struct{}{}
	boundaries[len(text)] = struct{}{}

	// Style openings at each position
	type styleEntry struct {
		span   StyleSpan
		marker StyleMarker
	}
	startsAt := make(map[int][]styleEntry)
	for _, s := range ir.Styles {
		m, ok := opts.Styles[s.Style]
		if !ok || s.Start >= s.End {
			continue
		}
		boundaries[s.Start] = struct{}{}
		boundaries[s.End] = struct{}{}
		startsAt[s.Start] = append(startsAt[s.Start], styleEntry{span: s, marker: m})
	}

	// Sort style entries at each position: wider spans first (outer wraps inner)
	for pos := range startsAt {
		entries := startsAt[pos]
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].span.End != entries[j].span.End {
				return entries[i].span.End > entries[j].span.End
			}
			return entries[i].span.Style < entries[j].span.Style
		})
		startsAt[pos] = entries
	}

	// Link openings
	type linkEntry struct {
		open  string
		close string
		end   int
	}
	linkStartsAt := make(map[int][]linkEntry)
	if opts.BuildLink != nil {
		for _, link := range ir.Links {
			if link.Start >= link.End {
				continue
			}
			open, close, ok := opts.BuildLink(link, text)
			if !ok {
				continue
			}
			boundaries[link.Start] = struct{}{}
			boundaries[link.End] = struct{}{}
			linkStartsAt[link.Start] = append(linkStartsAt[link.Start], linkEntry{
				open: open, close: close, end: link.End,
			})
		}
	}

	// Sort boundary points
	points := make([]int, 0, len(boundaries))
	for p := range boundaries {
		points = append(points, p)
	}
	sort.Ints(points)

	// Render using a stack for LIFO close ordering
	type stackItem struct {
		close string
		end   int
	}
	var stack []stackItem
	var out strings.Builder
	out.Grow(len(text) + len(text)/4) // estimate 25% overhead for markers

	for i, pos := range points {
		// Close items ending at this position (LIFO)
		for len(stack) > 0 && stack[len(stack)-1].end == pos {
			out.WriteString(stack[len(stack)-1].close)
			stack = stack[:len(stack)-1]
		}

		// Open links at this position
		if entries, ok := linkStartsAt[pos]; ok {
			for _, e := range entries {
				out.WriteString(e.open)
				stack = append(stack, stackItem{close: e.close, end: e.end})
			}
		}

		// Open styles at this position
		if entries, ok := startsAt[pos]; ok {
			for _, e := range entries {
				out.WriteString(e.marker.Open)
				stack = append(stack, stackItem{close: e.marker.Close, end: e.span.End})
			}
		}

		// Emit text segment
		if i+1 < len(points) {
			next := points[i+1]
			if next > pos {
				out.WriteString(escape(text[pos:next]))
			}
		}
	}

	return out.String()
}
