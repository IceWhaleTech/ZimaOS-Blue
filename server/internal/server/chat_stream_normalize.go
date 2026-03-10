package server

// trimLeadingReplyNewlines removes provider-emitted leading CR/LF tokens from
// the start of a streamed assistant reply. Many upstream models emit an empty
// first line before actual content; trimming it here keeps live rendering and
// persisted content consistent.
func trimLeadingReplyNewlines(existingContent, delta string) string {
	if existingContent != "" || delta == "" {
		return delta
	}

	trimIdx := 0
	for trimIdx < len(delta) {
		switch delta[trimIdx] {
		case '\n', '\r':
			trimIdx++
		default:
			return delta[trimIdx:]
		}
	}

	return ""
}
