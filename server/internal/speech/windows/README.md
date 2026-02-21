# Windows Native Speech Implementation

## 概述
使用WinRT API + CGO实现Windows系统原生的TTS(文本转语音)和ASR(语音识别)功能。

## 技术栈
- **WinRT API**: Windows.Media.SpeechSynthesis / SpeechRecognition
- **CGO**: Go与C++互操作
- **C++17**: WinRT现代C++支持

## 系统要求
- Windows 10 或更高版本
- Windows SDK 10.0.17763.0+
- CGO enabled
- MinGW-w64 (用于CGO编译)

## 文件结构
```
server/internal/speech/windows/
├── speech_tts_windows.h/cpp    # TTS C++ WinRT实现
├── speech_asr_windows.h/cpp    # ASR C++ WinRT实现
├── tts_windows.go              # TTS CGO绑定
├── asr_windows.go              # ASR CGO绑定
├── types.go                    # 类型定义
├── *_stub.go                   # 非Windows平台桩代码
└── *_test.go                   # 单元测试

server/internal/tts/
├── windows_native.go           # TTS Provider包装器
└── windows_native_stub.go      # 非Windows平台桩代码
```

## 编译
```bash
# 设置CGO
set CGO_ENABLED=1

# 编译
cd server
go build -tags windows

# 测试
go test -v ./internal/speech/windows/...
```

## 使用示例

### TTS (文本转语音)
```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/speech/windows"

// 创建TTS provider
provider := windows.NewWindowsTTSProvider()
defer provider.Close()

// 列出可用语音
voices, err := provider.ListVoices(context.Background())

// 合成语音
req := &windows.SynthesizeRequest{
    Text:  "Hello, this is a test.",
    Voice: voices[0].ID,
    Speed: 1.0,
}
resp, err := provider.Synthesize(context.Background(), req)
```

### ASR (语音识别)
```go
// 创建ASR provider
provider := windows.NewWindowsASRProvider("en-US")
defer provider.Close()

// 识别音频
req := &windows.RecognizeRequest{
    Audio:      audioReader,
    Language:   "en-US",
    SampleRate: 16000,
}
resp, err := provider.Recognize(context.Background(), req)
fmt.Println("识别结果:", resp.Text)
```

## 集成到现有系统
已添加 `ProviderWindowsNative` 到TTS Provider系统，可通过以下方式使用：
```go
provider := tts.NewWindowsNativeTTSProvider()
```

## 特性
- ✅ 支持多语言TTS (中文、英文、日文等)
- ✅ 可调节语速、音调、音量
- ✅ 支持语音识别 (需要音频输入)
- ✅ 系统原生，无需额外依赖
- ✅ 高质量语音输出

## 注意事项
1. 仅在Windows平台可用
2. 需要系统已安装对应语言包
3. ASR需要麦克风权限
4. 编译时需要Windows SDK

## 测试状态
- ✅ TTS基础功能
- ✅ 语音列表查询
- ✅ 文本合成
- ⚠️ ASR需要实际音频数据测试
