#!/usr/bin/env python3
"""Export SWE-Pruner (code-pruner) model to ONNX format.

Usage:
    pip install torch transformers safetensors onnx onnxruntime
    python scripts/export_pruner_onnx.py --model ayanami-kitasan/code-pruner --output pruner-model/

This exports:
    - model.onnx (the pruner model)
    - vocab.json, merges.txt, tokenizer.json, special_tokens_map.json (tokenizer files)
"""

import argparse
import json
import os
import shutil
import sys

import torch
import torch.nn as nn
from transformers import AutoModel, AutoTokenizer, AutoConfig


class PrunerONNXWrapper(nn.Module):
    """Simplified wrapper that extracts token-level scores from the pruner model.

    The original model has:
    - backbone (Qwen3-0.6B transformer)
    - multi-layer fusion (attention over early/middle/final hidden states)
    - CRF compression head

    For ONNX export, we export the backbone + fusion + emission scores (pre-CRF).
    CRF Viterbi decoding is implemented in Go since it's not easily exportable to ONNX.
    """

    def __init__(self, model):
        super().__init__()
        self.model = model

    def forward(self, input_ids, attention_mask):
        # Get token logits from the model's forward pass
        outputs = self.model(input_ids=input_ids, attention_mask=attention_mask)
        # token_logits shape: [batch_size, seq_len, num_labels] or [batch_size, seq_len]
        token_logits = outputs.token_logits
        if token_logits.dim() == 3:
            # Take the "keep" class probability (class 1)
            token_logits = token_logits[:, :, 1]
        return token_logits


def export_model(model_name: str, output_dir: str, max_length: int = 4096, opset: int = 17):
    os.makedirs(output_dir, exist_ok=True)

    print(f"Loading model from {model_name}...")

    # Try loading with trust_remote_code for custom architecture
    try:
        model = AutoModel.from_pretrained(
            model_name,
            trust_remote_code=True,
            torch_dtype=torch.float32,
        )
    except Exception as e:
        print(f"Failed to load with AutoModel: {e}")
        print("Trying manual loading...")
        # Fallback: load config and instantiate manually
        sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
        config = AutoConfig.from_pretrained(model_name, trust_remote_code=True)
        model = AutoModel.from_config(config, trust_remote_code=True)
        # Load weights
        from safetensors.torch import load_file
        state_dict = load_file(os.path.join(model_name, "model.safetensors"))
        model.load_state_dict(state_dict, strict=False)

    model.eval()

    # Load tokenizer
    print("Loading tokenizer...")
    tokenizer = AutoTokenizer.from_pretrained(model_name, trust_remote_code=True)

    # Save tokenizer files to output dir
    print("Saving tokenizer files...")
    tokenizer.save_pretrained(output_dir)

    # Also save config
    config = AutoConfig.from_pretrained(model_name, trust_remote_code=True)
    config.save_pretrained(output_dir)

    # Wrap model for ONNX export
    wrapper = PrunerONNXWrapper(model)
    wrapper.eval()

    # Create dummy inputs
    dummy_input_ids = torch.ones(1, max_length, dtype=torch.long)
    dummy_attention_mask = torch.ones(1, max_length, dtype=torch.long)

    # Export to ONNX
    onnx_path = os.path.join(output_dir, "model.onnx")
    print(f"Exporting ONNX model to {onnx_path} (max_length={max_length}, opset={opset})...")

    torch.onnx.export(
        wrapper,
        (dummy_input_ids, dummy_attention_mask),
        onnx_path,
        input_names=["input_ids", "attention_mask"],
        output_names=["token_scores"],
        dynamic_axes={
            "input_ids": {0: "batch_size", 1: "seq_len"},
            "attention_mask": {0: "batch_size", 1: "seq_len"},
            "token_scores": {0: "batch_size", 1: "seq_len"},
        },
        opset_version=opset,
        do_constant_folding=True,
    )

    # Verify ONNX model
    print("Verifying ONNX model...")
    import onnx
    onnx_model = onnx.load(onnx_path)
    onnx.checker.check_model(onnx_model)

    # Test with ONNX Runtime
    print("Testing with ONNX Runtime...")
    import onnxruntime as ort
    session = ort.InferenceSession(onnx_path)

    test_text = "def hello():\n    print('hello world')\n"
    tokens = tokenizer(test_text, return_tensors="np", padding="max_length",
                       max_length=max_length, truncation=True)

    outputs = session.run(None, {
        "input_ids": tokens["input_ids"],
        "attention_mask": tokens["attention_mask"],
    })

    token_scores = outputs[0]
    print(f"Output shape: {token_scores.shape}")
    print(f"Score range: [{token_scores.min():.4f}, {token_scores.max():.4f}]")

    # Report sizes
    model_size = os.path.getsize(onnx_path)
    print(f"\nExport complete!")
    print(f"  ONNX model: {model_size / 1024 / 1024:.1f} MB")
    print(f"  Output dir: {output_dir}")

    # List all files
    for f in sorted(os.listdir(output_dir)):
        fpath = os.path.join(output_dir, f)
        if os.path.isfile(fpath):
            size = os.path.getsize(fpath)
            print(f"  {f}: {size / 1024:.1f} KB" if size < 1024*1024 else f"  {f}: {size / 1024 / 1024:.1f} MB")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Export SWE-Pruner to ONNX")
    parser.add_argument("--model", default="ayanami-kitasan/code-pruner",
                        help="HuggingFace model name or local path")
    parser.add_argument("--output", default="pruner-model/",
                        help="Output directory for ONNX model and tokenizer")
    parser.add_argument("--max-length", type=int, default=4096,
                        help="Max sequence length for export")
    parser.add_argument("--opset", type=int, default=17,
                        help="ONNX opset version")
    args = parser.parse_args()

    export_model(args.model, args.output, args.max_length, args.opset)
