// Thai (ไทย)
// NOTE: Most strings inherit from en-US. Frequently used items are localized below.
import enUS from './en-US'

export default {
  ...enUS,
  speech: {
    ...enUS.speech,
    status: 'สถานะเสียงพูด',
    asr: 'การรู้จำเสียงพูด',
    tts: 'แปลงข้อความเป็นเสียงพูด',
    asrStatus: 'การรู้จำเสียงพูด',
    ttsStatus: 'แปลงข้อความเป็นเสียงพูด',
    currentASRModel: 'โมเดล ASR ปัจจุบัน',
    currentTTSModel: 'โมเดล TTS ปัจจุบัน',
    editBeforeSend: 'แก้ไขก่อนส่ง',
    editBeforeSendDesc: 'อนุญาตให้แก้ไขข้อความหลังจากรู้จำเสียงก่อนส่ง',
    asrModels: 'โมเดลรู้จำเสียงพูด',
    ttsModels: 'โมเดลแปลงข้อความเป็นเสียงพูด',
    downloading: 'กำลังดาวน์โหลด...',
    download: 'ดาวน์โหลด',
    use: 'ใช้',
    inUse: 'กำลังใช้งาน',
    noModels: 'ไม่มีโมเดลที่สามารถใช้ได้',
    asrModelInfo: {
      ...(enUS as any).speech?.asrModelInfo,
      whisperBase: {
        name: 'Whisper Base [แนะนำ]',
        description:
          'สมดุลที่ดีระหว่างความแม่นยำและประสิทธิภาพ เหมาะสำหรับงานรู้จำเสียงพูดทั่วไปส่วนใหญ่ แนะนำให้ใช้เป็นค่าเริ่มต้น.',
      },
    },
  },
}

