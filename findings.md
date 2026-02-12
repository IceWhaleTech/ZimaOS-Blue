# Findings: Chisel Research and Auto Mode Enhancement

## Chisel Research

### What is Chisel?
Chisel is a fast TCP/UDP tunnel over HTTP, secured via SSH. It's written in Go and consists of a single executable that includes both client and server components.

### Key Characteristics
- **Transport:** HTTP
- **Security:** SSH encryption
- **Use Case:** Bypassing firewalls, port forwarding, SOCKS proxying
- **Architecture:** Client-Server model (requires both components)

### Command Usage
```bash
# Server side
chisel server --port 8080 --reverse

# Client side
chisel client <server-url> <local-port>:<remote-port>
# Example: chisel client 172.16.0.10:8080 8000:80
```

### Critical Finding: No Public Servers
Unlike other providers in Auto mode, **Chisel does NOT have public servers available**:

| Provider | Public Server | Auto Mode Ready |
|----------|--------------|-----------------|
| Bore | bore.pub | ✅ Yes |
| Serveo | serveo.net | ✅ Yes |
| LocalTunnel | localtunnel.me | ✅ Yes |
| Cloudflared | Cloudflare infrastructure | ✅ Yes |
| **Chisel** | **None (requires own server)** | ❌ No |

### Implementation Decision
- Implement Chisel as a standalone provider (like Ngrok)
- Require users to configure their own Chisel server URL
- Do NOT add to Auto mode (no public server available)
- Add Cloudflared quick tunnel to Auto mode

## New Requirement: Provider Blacklist

### User Request
"如果bore打开后，立即关闭了，把bore加入黑名单（一天不再尝试），并且改用其他方式去，其他几个渠道也是"

Translation: If a provider opens and immediately closes, add it to a blacklist (don't try again for one day), and switch to other methods. Same for all channels.

### Requirements
1. Detect when a provider fails by opening and immediately closing
2. Add failed provider to a blacklist for 24 hours
3. Skip blacklisted providers in Auto mode
4. Apply to all providers (Bore, Serveo, LocalTunnel, Cloudflared)

### Implementation Approach
1. Create a blacklist storage mechanism (in-memory or persistent)
2. Track provider failures with timestamps
3. Check blacklist before attempting connection in Auto mode
4. Define "immediate close" threshold (e.g., < 5 seconds)
5. Automatically remove from blacklist after 24 hours

### Storage Options
- **In-memory:** Simple, but lost on restart
- **File-based:** Persistent across restarts (recommended)
- **Database:** Overkill for this use case

## Sources
- [jpillora/chisel GitHub](https://github.com/jpillora/chisel)
- [Chisel Tunneling Guide](https://0xdf.gitlab.io/cheatsheets/chisel)
- [Chisel Overview](https://book.izenynn.com/tools/chisel)
- [Chisel Documentation](https://www.hexmos.com/freedevtools/tldr/common/chisel/)
