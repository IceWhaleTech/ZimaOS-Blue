// Finnish (Suomi)
// NOTE: Most strings inherit from en-US. Frequently used items are localized below.
import enUS from './en-US'

export default {
  ...enUS,
  speech: {
    ...enUS.speech,
    status: 'Puhetilanne',
    asr: 'Puheentunnistus',
    tts: 'Tekstistä puheeksi',
    asrStatus: 'Puheentunnistus',
    ttsStatus: 'Tekstistä puheeksi',
    currentASRModel: 'Nykyinen ASR-malli',
    currentTTSModel: 'Nykyinen TTS-malli',
    editBeforeSend: 'Muokkaa ennen lähetystä',
    editBeforeSendDesc:
      'Salli tekstin muokkaaminen puheentunnistuksen jälkeen ennen lähettämistä',
    asrModels: 'Puheentunnistusmallit',
    ttsModels: 'Tekstistä puheeksi -mallit',
    downloading: 'Ladataan...',
    download: 'Lataa',
    use: 'Käytä',
    inUse: 'Käytössä',
    noModels: 'Ei saatavilla olevia malleja',
    asrModelInfo: {
      ...(enUS as any).speech?.asrModelInfo,
      whisperBase: {
        name: 'Whisper Base [Suositeltu]',
        description:
          'Hyvä tasapaino tarkkuuden ja suorituskyvyn välillä. Suositeltu useimpiin yleiskäyttöisiin puheentunnistusskenaarioihin.',
      },
    },
  },
}


