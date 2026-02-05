// Turkish (Türkçe)
// NOTE: Most strings inherit from en-US. Frequently used items are localized below.
import enUS from './en-US'

export default {
  ...enUS,
  speech: {
    ...enUS.speech,
    status: 'Konuşma durumu',
    asr: 'Konuşma tanıma',
    tts: 'Metinden konuşmaya',
    asrStatus: 'Konuşma tanıma',
    ttsStatus: 'Metinden konuşmaya',
    currentASRModel: 'Geçerli ASR modeli',
    currentTTSModel: 'Geçerli TTS modeli',
    editBeforeSend: 'Göndermeden önce düzenle',
    editBeforeSendDesc:
      'Konuşma tanıma sonrasında metni göndermeden önce düzenlemeye izin ver',
    asrModels: 'Konuşma tanıma modelleri',
    ttsModels: 'Metinden konuşmaya modelleri',
    downloading: 'İndiriliyor...',
    download: 'İndir',
    use: 'Kullan',
    inUse: 'Kullanımda',
    noModels: 'Kullanılabilir model yok',
    asrModelInfo: {
      ...(enUS as any).speech?.asrModelInfo,
      whisperBase: {
        name: 'Whisper Base [Önerilen]',
        description:
          'Doğruluk ve performans arasında iyi bir denge sunar. Çoğu genel amaçlı konuşma tanıma senaryosu için önerilir.',
      },
    },
  },
}


