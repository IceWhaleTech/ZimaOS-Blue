# harden

This module provides trial-activation hardening for ZimaOS-Blue's built-in LLM provider pool.

## Why does this exist?

ZimaOS-Blue ships with a free trial quota so new users can start chatting with AI immediately — no API key required. This hardening layer exists solely to prevent the trial resources from being abused, ensuring that the free quota remains available for everyone who wants to try the product.

It does **not** restrict any other functionality. Users who bring their own API keys are completely unaffected.

## 为什么需要这个模块？

ZimaOS-Blue 内置了免费试用额度，让新用户开箱即可体验 AI 对话，无需自行配置 API Key。此模块仅用于防止试用资源被滥用，以确保每位新用户都能顺利上手体验。

它**不会**限制任何其他功能。使用自有 API Key 的用户完全不受影响。

## Structure

- `harden.h` — C header declaring the public API
- `harden.c` — Implementation (activation marker read/write)
- `<os>_<arch>/libharden.a` — Pre-compiled static libraries for supported platforms
