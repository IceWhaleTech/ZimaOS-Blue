# Task Plan: Enhance Auto Remote Access with Cloudflared, Chisel, and Provider Blacklist

## Overview
1. Add cloudflared quick tunnel to Auto remote access mode
2. Implement Chisel as a standalone provider (requires user's own server)
3. Add provider blacklist mechanism to prevent repeated failures

## Current State Analysis
- ✅ Cloudflared quick tunnel is ALREADY implemented in `server/internal/tunnel/cloudflare.go`
  - When no CloudflareToken is provided, it uses quick tunnel mode (line 58-64)
  - Quick tunnel command: `cloudflared tunnel --url http://localhost:PORT`
- ❌ Chisel is NOT implemented yet (requires own server, no public servers available)
- ❌ Cloudflared is NOT included in Auto mode
- ❌ No blacklist mechanism for failed providers

## Implementation Phases

### Phase 1: Implement Provider Blacklist ✅
**Status:** Completed

**Purpose:** Prevent repeated attempts to use providers that fail immediately

**Tasks:**
1. ✅ Create `server/internal/tunnel/blacklist.go`
2. ✅ Implement blacklist storage (file-based for persistence)
3. ✅ Add blacklist check in Auto mode before trying providers
4. ✅ Detect "immediate close" failures (< 5 seconds)
5. ✅ Auto-expire blacklist entries after 24 hours

**Blacklist Logic:**
- Track provider failures with timestamps
- If provider closes within 5 seconds of starting, add to blacklist
- Skip blacklisted providers in Auto mode
- Persist blacklist across restarts

### Phase 2: Implement Chisel Provider ✅
**Status:** Completed

**Tasks:**
1. ✅ Create `server/internal/tunnel/chisel.go`
2. ✅ Implement Manager interface for Chisel
3. ✅ Add ProviderChisel constant to `provider.go`
4. ✅ Add Chisel to GetProviderInfos() in `provider.go`

**Chisel Details:**
- Chisel is a fast TCP/UDP tunnel over HTTP
- Client command: `chisel client <server-url> <local-port>:<remote-port>`
- **NOT for Auto mode** (requires user's own server)
- Standalone provider like Ngrok (requires configuration)

### Phase 3: Add Cloudflared to Auto Mode ✅
**Status:** Completed

**Tasks:**
1. ✅ Update `auto.go` providerOrder to include:
   - ProviderBore (existing)
   - ProviderServeo (existing)
   - ProviderLocalTunnel (existing)
   - ProviderCloudflare (new - quick tunnel mode)
2. ✅ Update switch statement in Start() to handle Cloudflare
3. ✅ Ensure cloudflared is called WITHOUT token for quick tunnel
4. ✅ Integrate blacklist check before trying each provider

### Phase 4: Update Provider Info ✅
**Status:** Completed

**Tasks:**
1. ✅ Update Auto provider description in GetProviderInfos()
2. ✅ Add Chisel to provider list as standalone option
3. ✅ Update documentation

### Phase 5: Testing ⏳
**Status:** Pending

**Tasks:**
1. Test Chisel provider standalone (with user's own server)
2. Test Auto mode with Cloudflared
3. Test blacklist mechanism (simulate immediate failures)
4. Verify blacklist persistence across restarts
5. Test parallel connection logic with blacklist

## Technical Decisions

### Chisel Implementation Approach
- Standalone provider only (NOT in Auto mode)
- Requires user to configure their own Chisel server URL
- Use exec.CommandContext to run chisel client
- Parse stdout/stderr for connection URL

### Cloudflared Auto Mode Integration
- Add to Auto mode parallel connection attempts
- Use quick tunnel mode (no token required)
- First successful connection wins
- Others are stopped automatically

### Blacklist Mechanism
- File-based storage: `~/.zimaos-blue/tunnel_blacklist.json`
- Structure: `{"provider": "bore", "blacklisted_at": "2026-02-02T10:00:00Z"}`
- Check on startup and before each Auto mode attempt
- Auto-expire after 24 hours

## Files to Modify
1. `server/internal/tunnel/provider.go` - Add ProviderChisel constant
2. `server/internal/tunnel/auto.go` - Add cloudflared to providerOrder, integrate blacklist
3. `server/internal/tunnel/blacklist.go` - NEW FILE - Implement blacklist mechanism
4. `server/internal/tunnel/chisel.go` - NEW FILE - Implement ChiselManager

## Files to Create
1. `server/internal/tunnel/blacklist.go` - Provider blacklist implementation
2. `server/internal/tunnel/chisel.go` - Chisel provider implementation

## Notes
- Follow TDD: Write tests first
- Cloudflared quick tunnel already exists, just needs Auto integration
- Chisel is standalone only (no public servers available)
- Blacklist prevents repeated failures for 24 hours
- Chisel requires user's own server configuration
