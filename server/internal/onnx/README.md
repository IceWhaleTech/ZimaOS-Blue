# Shared ONNX Runtime Package

This package provides a shared ONNX Runtime initialization and session management layer for ZimaOS-Blue.

## Purpose

Centralizes ONNX Runtime initialization to avoid duplicate initialization across multiple modules (pruner, TTS, etc.).

## Features

- **Thread-safe initialization**: `InitializeRuntime()` uses `sync.Once` to ensure ONNX Runtime is initialized only once
- **Two session types**:
  - `Session`: For models with fixed input/output shapes (pre-allocated tensors)
  - `DynamicSession`: For models with variable input shapes (dynamic tensors)

## Usage

### Fixed-shape models (like pruner)

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/onnx"

// Create pre-allocated tensors
inputTensor, _ := ort.NewEmptyTensor[int64](ort.NewShape(1, 4096))
outputTensor, _ := ort.NewEmptyTensor[float32](ort.NewShape(1, 4096))

// Create session with pre-allocated tensors
session, err := onnx.NewSession(
    modelPath,
    []string{"input"},
    []string{"output"},
    []ort.ArbitraryTensor{inputTensor},
    []ort.ArbitraryTensor{outputTensor},
)
defer session.Close()

// Run inference (no arguments needed)
err = session.Run()
```

### Variable-shape models (like TTS)

```go
import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/onnx"

// Create session for dynamic tensors
session, err := onnx.NewDynamicSession(
    modelPath,
    []string{"phonemes", "speaker_id", "speed", "pitch"},
    []string{"audio"},
)
defer session.Close()

// Create input tensors dynamically
phonemesTensor, _ := ort.NewTensor(ort.NewShape(1, len(phonemes)), phonemes)
speakerTensor, _ := ort.NewTensor(ort.NewShape(1), []int64{speakerID})
outputTensor, _ := ort.NewEmptyTensor[float32](ort.NewShape(1, 0))

// Run inference with dynamic tensors
err = session.Run(
    []ort.Value{phonemesTensor, speakerTensor, ...},
    []ort.Value{outputTensor},
)
```

## Modules Using This Package

- `internal/pruner`: Neural code pruning (fixed-shape)
- `internal/tts`: Supertonic TTS provider (variable-shape)

## Implementation Notes

- `Session` wraps `ort.AdvancedSession` for pre-allocated tensor workflows
- `DynamicSession` wraps `ort.DynamicAdvancedSession` for dynamic tensor workflows
- Both provide a unified `Close()` method for resource cleanup
- ONNX Runtime initialization is idempotent and thread-safe
