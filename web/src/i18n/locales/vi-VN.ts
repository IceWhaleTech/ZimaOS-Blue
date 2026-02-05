// Vietnamese (Tiếng Việt)
// NOTE: Most strings inherit from en-US. Frequently used items are localized below.
import enUS from './en-US'

export default {
  ...enUS,
  speech: {
    ...enUS.speech,
    status: 'Trạng thái giọng nói',
    asr: 'Nhận dạng giọng nói',
    tts: 'Chuyển văn bản thành giọng nói',
    asrStatus: 'Nhận dạng giọng nói',
    ttsStatus: 'Chuyển văn bản thành giọng nói',
    currentASRModel: 'Mô hình ASR hiện tại',
    currentTTSModel: 'Mô hình TTS hiện tại',
    editBeforeSend: 'Chỉnh sửa trước khi gửi',
    editBeforeSendDesc: 'Cho phép chỉnh sửa văn bản sau khi nhận dạng giọng nói trước khi gửi',
    asrModels: 'Các mô hình nhận dạng giọng nói',
    ttsModels: 'Các mô hình chuyển văn bản thành giọng nói',
    downloading: 'Đang tải xuống...',
    download: 'Tải xuống',
    use: 'Sử dụng',
    inUse: 'Đang sử dụng',
    noModels: 'Không có mô hình khả dụng',
    asrModelInfo: {
      ...(enUS as any).speech?.asrModelInfo,
      whisperBase: {
        name: 'Whisper Base [Đề xuất]',
        description:
          'Cân bằng tốt giữa độ chính xác và hiệu năng. Được khuyến nghị cho hầu hết các kịch bản nhận dạng giọng nói phổ thông.',
      },
    },
  },
}

