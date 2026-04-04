package tools

import "sync/atomic"

type ToolSurfaceAuditSnapshot struct {
	AliasRewriteCount      uint64 `json:"tool_surface_alias_rewrite_count"`
	CacheInvalidationCount uint64 `json:"tool_surface_cache_invalidation_count"`
	ExecCutoverCount       uint64 `json:"tool_surface_exec_cutover_count"`
}

type ToolSurfaceAuditState struct {
	aliasRewriteCount      atomic.Uint64
	cacheInvalidationCount atomic.Uint64
	execCutoverCount       atomic.Uint64
}

func NewToolSurfaceAuditState() *ToolSurfaceAuditState {
	return &ToolSurfaceAuditState{}
}

func (s *ToolSurfaceAuditState) RecordAliasRewrite() {
	if s == nil {
		return
	}
	s.aliasRewriteCount.Add(1)
}

func (s *ToolSurfaceAuditState) RecordCacheInvalidation() {
	if s == nil {
		return
	}
	s.cacheInvalidationCount.Add(1)
}

func (s *ToolSurfaceAuditState) RecordExecCutover() {
	if s == nil {
		return
	}
	s.execCutoverCount.Add(1)
}

func (s *ToolSurfaceAuditState) Snapshot() ToolSurfaceAuditSnapshot {
	if s == nil {
		return ToolSurfaceAuditSnapshot{}
	}
	return ToolSurfaceAuditSnapshot{
		AliasRewriteCount:      s.aliasRewriteCount.Load(),
		CacheInvalidationCount: s.cacheInvalidationCount.Load(),
		ExecCutoverCount:       s.execCutoverCount.Load(),
	}
}
