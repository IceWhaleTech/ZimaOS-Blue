// English (UK)
import enUS from './en-US'

export default {
  ...enUS,
  localeNames: {
    ...enUS.localeNames,
    'en-GB': 'English (UK)',
    'en-US': 'English (US)',
  },
  // Note: Most British English spelling differences (colour, favourite, etc.)
  // are handled in the UI components or are not used in the codebase.
  // This file can be extended with British English translations as needed.
    personality: {
    ...enUS.personality,
  },
  memoryService: {
    ...enUS.memoryService,
  },
}
