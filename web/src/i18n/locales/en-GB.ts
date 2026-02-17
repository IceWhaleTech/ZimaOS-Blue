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
  tools: {
    names: {
      'Web Search': 'Web Search',
      'Calculator': 'Calculator',
      'System Info': 'System Info',
      'Current Time': 'Current Time',
      'File Read': 'File Read',
      'File Write': 'File Write',
      'Memory Search': 'Memory Search',
      'Memory Store': 'Memory Store',
      'Memory Get': 'Memory Get',
      'Memory Stats': 'Memory Stats',
    },
    params: {
      query: 'Query',
      region: 'Region',
      max_results: 'Max Results',
      path: 'Path',
      content: 'Content',
      filename: 'Filename',
      expression: 'Expression',
      keyword: 'Keyword',
      limit: 'Limit',
      offset: 'Offset',
      id: 'ID',
    },
    calling: 'Calling Tools',
    callingProgress: 'Calling Tools...',
    callCount: 'Tool Calls ({count})',
  },
  thinking: {
    title: 'Thinking Process',
    expand: 'Expand',
    inProgress: 'Thinking...',
  },
  search: {
    resultCount: '{count} results',
  },
  chat: {
    ...enUS.chat,
    startRecording: 'Start recording',
    stopRecording: 'Stop recording',
    recording: 'Recording...',
    holdToSpeak: 'Hold to speak',
    releaseToSend: 'Release to send',
    clickToRecord: 'Click to record',
    switchToKeyboard: 'Switch to keyboard',
    switchToVoice: 'Switch to voice',
    voiceTranscribing: 'Transcribing...',
    voiceTranscriptionError: 'Failed to transcribe audio',
    voiceRecordingError: 'A recording error occurred',
    voiceMicrophoneError: 'Could not access microphone',
    dismiss: 'Dismiss',
    transcription: {
      title: 'Transcription',
      placeholder: 'Transcribed text will appear here...',
      send: 'Send',
      confidence: '{percent}% confidence',
      hint: 'Press Ctrl+Enter to send, Esc to cancel',
    },
  },
}
