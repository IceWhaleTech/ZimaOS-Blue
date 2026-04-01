# Cutover Status Report - 2026-04-01

## Summary

已恢复 workflow 文件并修复关键编译错误。当前 cutover 进度如下：

### 门禁状态

| Gate | Status | Details |
|------|--------|---------|
| **Selector Gate** | ✅ PASS | Q4 v2: pass_rate=1.0, route_compatible_rate=1.0, critical_regression=0 |
| **Budget Gate** | ✅ PASS | schema_reduction=83.4%, latency_increase=0% |
| **Execution Gate** | ❌ FAIL | 需要修复 provider 配置和 rate limiting |

### Execution Gate 失败分析

- **总用例**: 85
- **通过**: 0
- **失败**: 54 (run_failed: 30, verification_failed: 27)
- **错误**: 31 (missing_evidence_collection: 27, infra_provider_blocked: 1)

### 关键阻塞点

1. **Provider 配置**: 需要正确配置可用的 LLM provider
2. **Rate Limiting**: 需要 backoff 和重试策略
3. **JWT 认证**: HTTP API 需要 JWT token

### 已完成的修复

1. ✅ 补回 5 个 GitHub Actions workflow 文件
2. ✅ 修复编译错误 (移除未使用的 sync import)
3. ✅ 配置 Lanyi provider (http://1.95.142.151:3000)
4. ✅ 更新文档中 Batch 迁移状态

### 下一步建议

1. **完成 Provider 配置**: 配置 API key 和 model
2. **重新运行 Execution Gate**: 使用 `--skip-hil` 模式
3. **验证三门禁都通过**: 达到 cutover 标准

