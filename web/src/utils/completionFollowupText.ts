const ENGLISH_COMPLETION_FOLLOWUP_HEADING = "If you'd like, I can also help with:"
const ENGLISH_COMPLETION_FOLLOWUP_HEADING_LINE_RE =
  /^([ \t]*)If you'd like, I can also help with:\s*$/gm

export function localizeCompletionFollowupHeading(
  content: string,
  localizedHeading: string
): string {
  if (!content || !localizedHeading) return content
  if (!content.includes(ENGLISH_COMPLETION_FOLLOWUP_HEADING)) return content

  return content.replace(
    ENGLISH_COMPLETION_FOLLOWUP_HEADING_LINE_RE,
    (_match, indentation: string) => `${indentation}${localizedHeading}`
  )
}
