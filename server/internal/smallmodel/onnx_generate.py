#!/usr/bin/env python3
"""Small-model ONNX text generation helper.

Input JSON (via --input):
{
  "model_dir": "...",
  "prompt": "...",
  "max_tokens": 96,
  "temperature": 0.2,
  "device": "cpu|gpu"
}
"""

import argparse
import base64
import json
import os
import sys
import tempfile


def _load_request(path: str) -> dict:
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def _build_model(og, model_dir: str, device: str):
    # onnxruntime-genai accepts either a config path or Config object.
    config_path = os.path.join(model_dir, "config.json")
    try:
        cfg = og.Config(config_path)
        if hasattr(cfg, "clear_providers"):
            cfg.clear_providers()
        if hasattr(cfg, "append_provider"):
            if device == "gpu":
                if sys.platform == "darwin":
                    # macOS GPU path
                    cfg.append_provider("CoreMLExecutionProvider")
                else:
                    cfg.append_provider("CUDAExecutionProvider")
            else:
                cfg.append_provider("CPUExecutionProvider")
        return og.Model(cfg)
    except Exception:
        # Fallback to default provider selection.
        return og.Model(config_path)


def _to_list(x):
    if hasattr(x, "tolist"):
        return x.tolist()
    return list(x)


def _image_suffix(mime_type: str) -> str:
    mt = (mime_type or "").lower()
    if "jpeg" in mt or "jpg" in mt:
        return ".jpg"
    if "webp" in mt:
        return ".webp"
    if "gif" in mt:
        return ".gif"
    if "bmp" in mt:
        return ".bmp"
    return ".png"


def _materialize_images(images: list[dict]) -> list[str]:
    paths = []
    for img in images:
        data = (img or {}).get("data") or ""
        if not data:
            continue
        raw = base64.b64decode(data)
        suffix = _image_suffix((img or {}).get("mime_type") or "")
        with tempfile.NamedTemporaryFile(prefix="sm-img-", suffix=suffix, delete=False) as f:
            f.write(raw)
            paths.append(f.name)
    return paths


def _build_mm_prompt(prompt: str, image_count: int) -> str:
    # Qwen-style image placeholder. Keep fallback safe if model ignores it.
    placeholders = "".join("<|vision_start|><|image_pad|><|vision_end|>\n" for _ in range(image_count))
    return f"<|im_start|>user\n{placeholders}{prompt}<|im_end|>\n<|im_start|>assistant\n"


def _prepare_multimodal_inputs(og, model, params, prompt: str, image_paths: list[str]):
    if not image_paths:
        return False

    if not hasattr(model, "create_multimodal_processor"):
        return False
    if not hasattr(params, "set_inputs"):
        return False

    processor = model.create_multimodal_processor()
    mm_prompt = _build_mm_prompt(prompt, len(image_paths))
    inputs = None

    images_obj = None
    if hasattr(og, "Images") and hasattr(og.Images, "open"):
        try:
            images_obj = og.Images.open(*image_paths)
        except Exception:
            try:
                images_obj = og.Images.open(image_paths)
            except Exception:
                images_obj = None

    candidates = []
    if images_obj is not None:
        candidates.append(lambda: processor(mm_prompt, images=images_obj))
        candidates.append(lambda: processor(mm_prompt, images_obj))
    candidates.append(lambda: processor(mm_prompt, images=image_paths))
    candidates.append(lambda: processor(mm_prompt, image_paths))
    candidates.append(lambda: processor(mm_prompt))

    for call in candidates:
        try:
            inputs = call()
            if inputs is not None:
                break
        except Exception:
            continue

    if inputs is None:
        return False

    params.set_inputs(inputs)
    return True


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", required=True)
    args = parser.parse_args()

    req = _load_request(args.input)
    model_dir = req.get("model_dir") or ""
    prompt = (req.get("prompt") or "").strip()
    max_tokens = int(req.get("max_tokens") or 128)
    temperature = float(req.get("temperature") or 0.2)
    device = (req.get("device") or "cpu").strip().lower()
    images = req.get("images") or []

    if not model_dir:
        print("missing model_dir", file=sys.stderr)
        return 2
    if not prompt:
        print("missing prompt", file=sys.stderr)
        return 2
    if max_tokens <= 0:
        max_tokens = 128
    if max_tokens > 1024:
        max_tokens = 1024
    if temperature < 0:
        temperature = 0.0
    if temperature > 2:
        temperature = 2.0

    try:
        import onnxruntime_genai as og

        model = _build_model(og, model_dir, device)
        params = og.GeneratorParams(model)
        if hasattr(params, "set_search_options"):
            params.set_search_options(
                max_length=max_tokens + 2048,
                do_sample=temperature > 0.0,
                temperature=temperature,
            )

        image_paths = _materialize_images(images) if images else []
        try:
            multimodal_ok = _prepare_multimodal_inputs(og, model, params, prompt, image_paths)

            tokenizer = og.Tokenizer(model)
            prompt_ids = []
            if not multimodal_ok:
                prompt_ids = _to_list(tokenizer.encode(prompt))
        finally:
            for p in image_paths:
                try:
                    os.remove(p)
                except Exception:
                    pass

        generator = og.Generator(model, params)
        if prompt_ids:
            try:
                import numpy as np

                generator.append_tokens(np.asarray(prompt_ids, dtype=np.int32))
            except Exception:
                generator.append_tokens(prompt_ids)

        while not generator.is_done():
            generator.generate_next_token()

        all_ids = _to_list(generator.get_sequence(0))
        new_ids = all_ids[len(prompt_ids):]
        text = tokenizer.decode(new_ids).strip()
        print(text)
        return 0
    except Exception as e:
        print(f"onnx runtime error: {e}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
