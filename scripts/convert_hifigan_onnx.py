#!/usr/bin/env python3
"""Convert HiFi-GAN V3 PyTorch checkpoint to ONNX format.

Usage:
    # 1. Download pretrained LJ_V3 from Google Drive:
    # https://drive.google.com/drive/folders/1-eEYTB5Av9jNql0WGBlRoi-WH2J7bp5Y

    # 2. Run conversion:
    pip install torch numpy
    python scripts/convert_hifigan_onnx.py \
        --checkpoint /path/to/LJ_V3/generator_v3 \
        --config /path/to/LJ_V3/config.json \
        --output hifigan_v3.onnx

    # V3 config: upsample_rates=[8,8,4], resblock="2"
    # Input: mel [1, 80, T], Output: audio [1, 1, T*256]
    # Mel params: sr=22050, n_fft=1024, hop=256, win=1024, n_mels=80, fmin=0, fmax=8000
"""

import argparse
import json
import sys
import os

import torch
import torch.nn as nn
import torch.nn.functional as F
from torch.nn import Conv1d, ConvTranspose1d
from torch.nn.utils import remove_weight_norm, weight_norm

LRELU_SLOPE = 0.1


def init_weights(m, mean=0.0, std=0.01):
    if isinstance(m, (Conv1d, ConvTranspose1d)):
        m.weight.data.normal_(mean, std)


def get_padding(kernel_size, dilation=1):
    return int((kernel_size * dilation - dilation) / 2)


class ResBlock1(nn.Module):
    def __init__(self, channels, kernel_size=3, dilation=(1, 3, 5)):
        super().__init__()
        self.convs1 = nn.ModuleList([
            weight_norm(Conv1d(channels, channels, kernel_size, 1, dilation=d,
                               padding=get_padding(kernel_size, d)))
            for d in dilation
        ])
        self.convs1.apply(init_weights)
        self.convs2 = nn.ModuleList([
            weight_norm(Conv1d(channels, channels, kernel_size, 1, dilation=1,
                               padding=get_padding(kernel_size, 1)))
            for _ in dilation
        ])
        self.convs2.apply(init_weights)

    def forward(self, x):
        for c1, c2 in zip(self.convs1, self.convs2):
            xt = F.leaky_relu(x, LRELU_SLOPE)
            xt = c1(xt)
            xt = F.leaky_relu(xt, LRELU_SLOPE)
            xt = c2(xt)
            x = xt + x
        return x

    def remove_weight_norm(self):
        for l in self.convs1:
            remove_weight_norm(l)
        for l in self.convs2:
            remove_weight_norm(l)


class ResBlock2(nn.Module):
    def __init__(self, channels, kernel_size=3, dilation=(1, 3)):
        super().__init__()
        self.convs = nn.ModuleList([
            weight_norm(Conv1d(channels, channels, kernel_size, 1, dilation=d,
                               padding=get_padding(kernel_size, d)))
            for d in dilation
        ])
        self.convs.apply(init_weights)

    def forward(self, x):
        for c in self.convs:
            xt = F.leaky_relu(x, LRELU_SLOPE)
            xt = c(xt)
            x = xt + x
        return x

    def remove_weight_norm(self):
        for l in self.convs:
            remove_weight_norm(l)


class Generator(nn.Module):
    def __init__(self, h):
        super().__init__()
        self.h = h
        self.num_kernels = len(h["resblock_kernel_sizes"])
        self.num_upsamples = len(h["upsample_rates"])
        self.conv_pre = weight_norm(
            Conv1d(80, h["upsample_initial_channel"], 7, 1, padding=3))

        resblock = ResBlock1 if h["resblock"] == "1" else ResBlock2

        self.ups = nn.ModuleList()
        for i, (u, k) in enumerate(zip(h["upsample_rates"], h["upsample_kernel_sizes"])):
            ch = h["upsample_initial_channel"] // (2 ** (i + 1))
            self.ups.append(weight_norm(
                ConvTranspose1d(ch * 2, ch, k, u, padding=(k - u) // 2)))

        self.resblocks = nn.ModuleList()
        for i in range(len(self.ups)):
            ch = h["upsample_initial_channel"] // (2 ** (i + 1))
            for j, (k, d) in enumerate(zip(h["resblock_kernel_sizes"],
                                            h["resblock_dilation_sizes"])):
                self.resblocks.append(resblock(ch, k, d))

        self.conv_post = weight_norm(Conv1d(ch, 1, 7, 1, padding=3))
        self.ups.apply(init_weights)
        self.conv_post.apply(init_weights)

    def forward(self, x):
        x = self.conv_pre(x)
        for i in range(self.num_upsamples):
            x = F.leaky_relu(x, LRELU_SLOPE)
            x = self.ups[i](x)
            xs = None
            for j in range(self.num_kernels):
                if xs is None:
                    xs = self.resblocks[i * self.num_kernels + j](x)
                else:
                    xs += self.resblocks[i * self.num_kernels + j](x)
            x = xs / self.num_kernels
        x = F.leaky_relu(x)
        x = self.conv_post(x)
        x = torch.tanh(x)
        return x

    def remove_weight_norm(self):
        for l in self.ups:
            remove_weight_norm(l)
        for l in self.resblocks:
            l.remove_weight_norm()
        remove_weight_norm(self.conv_pre)
        remove_weight_norm(self.conv_post)


def main():
    parser = argparse.ArgumentParser(description="Convert HiFi-GAN to ONNX")
    parser.add_argument("--checkpoint", required=True, help="Path to generator checkpoint (e.g. g_02500000)")
    parser.add_argument("--config", required=True, help="Path to config.json")
    parser.add_argument("--output", default="hifigan.onnx", help="Output ONNX path")
    args = parser.parse_args()

    with open(args.config) as f:
        h = json.load(f)

    model = Generator(h)
    state_dict = torch.load(args.checkpoint, map_location="cpu", weights_only=True)
    if "generator" in state_dict:
        state_dict = state_dict["generator"]
    model.load_state_dict(state_dict)
    model.eval()
    model.remove_weight_norm()

    # Input: [batch=1, mel_channels=80, time_steps]
    dummy_input = torch.randn(1, 80, 100)

    torch.onnx.export(
        model,
        dummy_input,
        args.output,
        input_names=["mel"],
        output_names=["audio"],
        dynamic_axes={
            "mel": {2: "time_steps"},
            "audio": {2: "audio_length"},
        },
        opset_version=17,
        do_constant_folding=True,
    )
    print(f"Exported to {args.output}")
    print(f"  Input:  mel [1, 80, T]")
    print(f"  Output: audio [1, 1, T*{torch.prod(torch.tensor(h['upsample_rates'])).item()}]")

    size_mb = os.path.getsize(args.output) / 1024 / 1024
    print(f"  Size:   {size_mb:.1f} MB")


if __name__ == "__main__":
    main()
