// Hindi (हिन्दी)
// NOTE: Most strings inherit from en-US. Frequently used items are localized below.
import enUS from './en-US'

export default {
  ...enUS,
  speech: {
    ...enUS.speech,
    status: 'वॉइस स्थिति',
    asr: 'वाक् पहचान',
    tts: 'पाठ से वाणी',
    asrStatus: 'वाक् पहचान',
    ttsStatus: 'पाठ से वाणी',
    currentASRModel: 'वर्तमान ASR मॉडल',
    currentTTSModel: 'वर्तमान TTS मॉडल',
    editBeforeSend: 'भेजने से पहले संपादित करें',
    editBeforeSendDesc:
      'वाक् पहचान के बाद पाठ को भेजने से पहले संपादित करने की अनुमति दें',
    asrModels: 'वाक् पहचान मॉडल',
    ttsModels: 'पाठ से वाणी मॉडल',
    downloading: 'डाउनलोड हो रहा है...',
    download: 'डाउनलोड',
    use: 'उपयोग करें',
    inUse: 'प्रयोग में है',
    noModels: 'कोई उपलब्ध मॉडल नहीं है',
    asrModelInfo: {
      ...(enUS as any).speech?.asrModelInfo,
      whisperBase: {
        name: 'Whisper Base [अनुशंसित]',
        description:
          'सटीकता और प्रदर्शन के बीच अच्छा संतुलन। अधिकांश सामान्य वाक्-पहचान परिदृश्यों के लिए अनुशंसित।',
      },
    },
  },
}


