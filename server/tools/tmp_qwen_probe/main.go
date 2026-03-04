package main

import (
    "fmt"
    "path/filepath"

    "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/onnx"
)

func decoderInputNames() []string {
    names := []string{"inputs_embeds", "attention_mask", "position_ids"}
    for i := 0; i < 24; i++ {
        if i%4 == 3 {
            names = append(names, fmt.Sprintf("past_key_values.%d.key", i), fmt.Sprintf("past_key_values.%d.value", i))
            continue
        }
        names = append(names, fmt.Sprintf("past_conv.%d", i), fmt.Sprintf("past_recurrent.%d", i))
    }
    return names
}

func main() {
    modelDir := "/Users/orca/.zimaos-blue/data/models/qwen3.5-0.8b-onnx-q4"
    dataDir := filepath.Dir(filepath.Dir(modelDir))

    onnx.SetDataDir(dataDir)
    if lib := onnx.RuntimeLibPath(dataDir); lib != "" {
        onnx.SetLibraryPath(lib)
    } else if lib, err := onnx.EnsureRuntime(dataDir); err == nil {
        onnx.SetLibraryPath(lib)
    } else {
        fmt.Printf("runtime ensure failed: %v\n", err)
    }

    embedPath := filepath.Join(modelDir, "onnx", "embed_tokens_q4.onnx")
    decPath := filepath.Join(modelDir, "onnx", "decoder_model_merged_q4.onnx")
    visPath := filepath.Join(modelDir, "onnx", "vision_encoder_q4.onnx")

    s1, err := onnx.NewDynamicSession(embedPath, []string{"input_ids"}, []string{"inputs_embeds"})
    fmt.Printf("embed session: err=%v\n", err)
    if s1 != nil {
        _ = s1.Close()
    }

    s2, err := onnx.NewDynamicSession(decPath, decoderInputNames(), []string{"logits"})
    fmt.Printf("decoder session: err=%v\n", err)
    if s2 != nil {
        _ = s2.Close()
    }

    s3, err := onnx.NewDynamicSession(visPath, []string{"pixel_values", "image_grid_thw"}, []string{"image_features"})
    fmt.Printf("vision session: err=%v\n", err)
    if s3 != nil {
        _ = s3.Close()
    }
}

