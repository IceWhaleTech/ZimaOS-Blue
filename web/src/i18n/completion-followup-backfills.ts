import type { LocaleKey } from './locale-catalog'

function buildCompletionFollowupBackfill(completionFollowupHeading: string) {
  return {
    chat: {
      completionFollowupHeading,
    },
  }
}

const completionFollowupBackfills: Partial<Record<LocaleKey, object>> = {
  'ca-ES': buildCompletionFollowupBackfill('Si vols, també et puc ajudar amb:'),
  'cs-CZ': buildCompletionFollowupBackfill('Pokud chcete, mohu vám také pomoci s:'),
  'da-DK': buildCompletionFollowupBackfill('Hvis du vil, kan jeg også hjælpe med:'),
  'de-DE': buildCompletionFollowupBackfill(
    'Wenn Sie möchten, kann ich Ihnen auch bei Folgendem helfen:'
  ),
  'el-GR': buildCompletionFollowupBackfill('Αν θέλετε, μπορώ επίσης να βοηθήσω με:'),
  'en-GB': buildCompletionFollowupBackfill("If you'd like, I can also help with:"),
  'en-US': buildCompletionFollowupBackfill("If you'd like, I can also help with:"),
  'es-ES': buildCompletionFollowupBackfill('Si quieres, también puedo ayudarte con:'),
  'fr-FR': buildCompletionFollowupBackfill('Si vous le souhaitez, je peux aussi vous aider avec :'),
  'ga-IE': buildCompletionFollowupBackfill('Más mian leat, is féidir liom cabhrú leat freisin le:'),
  'hr-HR': buildCompletionFollowupBackfill('Ako želite, mogu vam također pomoći s:'),
  'hu-HU': buildCompletionFollowupBackfill('Ha szeretné, ezekben is tudok segíteni:'),
  'it-IT': buildCompletionFollowupBackfill('Se vuoi, posso anche aiutarti con:'),
  'ja-JP': buildCompletionFollowupBackfill('必要であれば、次のこともお手伝いできます:'),
  'ko-KR': buildCompletionFollowupBackfill('원하시면 다음도 도와드릴 수 있습니다:'),
  'ml-IN': buildCompletionFollowupBackfill('താൽപ്പര്യമുണ്ടെങ്കിൽ, ഇതിലും ഞാൻ സഹായിക്കാം:'),
  'nb-NO': buildCompletionFollowupBackfill('Hvis du vil, kan jeg også hjelpe med:'),
  'nl-NL': buildCompletionFollowupBackfill('Als je wilt, kan ik ook helpen met:'),
  'pl-PL': buildCompletionFollowupBackfill('Jeśli chcesz, mogę też pomóc w:'),
  'pt-BR': buildCompletionFollowupBackfill('Se quiser, também posso ajudar com:'),
  'pt-PT': buildCompletionFollowupBackfill('Se quiser, também posso ajudar com:'),
  'ro-RO': buildCompletionFollowupBackfill('Dacă doriți, vă mai pot ajuta și cu:'),
  'ru-RU': buildCompletionFollowupBackfill('Если хотите, я также могу помочь с:'),
  'sk-SK': buildCompletionFollowupBackfill('Ak chcete, môžem vám tiež pomôcť s:'),
  'sv-SE': buildCompletionFollowupBackfill('Om du vill kan jag också hjälpa till med:'),
  'zh-CN': buildCompletionFollowupBackfill('如果你愿意，我还可以帮你：'),
  'zh-TW': buildCompletionFollowupBackfill('如果你願意，我還可以幫你：'),
}

export default completionFollowupBackfills
