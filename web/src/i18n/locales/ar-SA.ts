// Arabic (العربية)
// NOTE: Most strings inherit from en-US. Frequently used items are localized below.
import enUS from './en-US'

export default {
  ...enUS,
  speech: {
    ...enUS.speech,
    status: 'حالة الصوت',
    asr: 'التعرف على الكلام',
    tts: 'تحويل النص إلى كلام',
    asrStatus: 'التعرف على الكلام',
    ttsStatus: 'تحويل النص إلى كلام',
    currentASRModel: 'نموذج ASR الحالي',
    currentTTSModel: 'نموذج TTS الحالي',
    editBeforeSend: 'تحرير قبل الإرسال',
    editBeforeSendDesc: 'السماح بتحرير النص بعد التعرف على الكلام قبل إرساله',
    asrModels: 'نماذج التعرف على الكلام',
    ttsModels: 'نماذج تحويل النص إلى كلام',
    downloading: 'جارٍ التنزيل...',
    download: 'تنزيل',
    use: 'استخدام',
    inUse: 'قيد الاستخدام',
    noModels: 'لا توجد نماذج متاحة',
    asrModelInfo: {
      ...(enUS as any).speech?.asrModelInfo,
      whisperBase: {
        name: 'Whisper Base [موصى به]',
        description:
          'توازن جيد بين الدقة والأداء. موصى به لمعظم سيناريوهات التعرف على الكلام للاستخدامات العامة.',
      },
    },
  },
}

