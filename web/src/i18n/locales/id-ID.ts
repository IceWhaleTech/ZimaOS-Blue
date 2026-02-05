// Indonesian (Bahasa Indonesia)
// NOTE: Most strings inherit from en-US. Frequently used items are localized below.
import enUS from './en-US'

export default {
  ...enUS,
  speech: {
    ...enUS.speech,
    status: 'Status Suara',
    asr: 'Pengenalan Suara',
    tts: 'Teks-ke-Ucapan',
    asrStatus: 'Pengenalan Suara',
    ttsStatus: 'Teks-ke-Ucapan',
    currentASRModel: 'Model ASR Saat Ini',
    currentTTSModel: 'Model TTS Saat Ini',
    editBeforeSend: 'Edit sebelum kirim',
    editBeforeSendDesc: 'Izinkan mengedit teks setelah pengenalan suara sebelum dikirim',
    asrModels: 'Model pengenalan suara',
    ttsModels: 'Model teks-ke-ucapan',
    downloading: 'Mengunduh...',
    download: 'Unduh',
    use: 'Gunakan',
    inUse: 'Sedang digunakan',
    noModels: 'Tidak ada model yang tersedia',
    asrModelInfo: {
      ...(enUS as any).speech?.asrModelInfo,
      whisperBase: {
        name: 'Whisper Base [Direkomendasikan]',
        description:
          'Keseimbangan yang baik antara akurasi dan performa. Direkomendasikan untuk sebagian besar skenario pengenalan suara umum.',
      },
    },
  },
}

