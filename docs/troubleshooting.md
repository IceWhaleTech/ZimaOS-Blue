# Troubleshooting Guide

[中文版本](./zh/troubleshooting.md)

This guide helps you diagnose and resolve common issues with ZimaOS-Echo.

## Quick Diagnostics

### Check Service Status

```bash
# Systemd service
sudo systemctl status echo

# Docker container
docker ps | grep echo
docker logs zimaos-echo --tail 100

# Direct process
ps aux | grep echo
```

### Check Logs

```bash
# Systemd logs
sudo journalctl -u echo -f

# Log file (if configured)
tail -f /var/log/echo/echo.log

# Docker logs
docker logs -f zimaos-echo
```

### Health Check

```bash
curl http://localhost:8080/health
```

Expected response:
```json
{"status":"ok","version":"0.5.0"}
```

---

## Common Issues

### Service Won't Start

#### Port Already in Use

**Symptom:** Error message "address already in use"

**Solution:**
```bash
# Find process using port 8080
sudo lsof -i :8080
# or
sudo netstat -tlnp | grep 8080

# Kill the process or change Echo's port
# In config.yaml:
server:
  port: 8081
```

#### Missing Configuration

**Symptom:** "config file not found" or "missing required field"

**Solution:**
```bash
# Create minimal config
cat > /etc/echo/config.yaml << 'EOF'
server:
  port: 8080
llm:
  default_provider: "ollama"
  providers:
    ollama:
      base_url: "http://localhost:11434"
      model: "llama2"
auth:
  enabled: false
EOF
```

#### Missing JWT Secret

**Symptom:** "JWT secret is required" error

**Solution:**
```bash
# Generate and set JWT secret
export JWT_SECRET=$(openssl rand -base64 32)

# Or add to config
auth:
  jwt:
    secret: "your-32-character-secret-here"
```

#### Permission Denied

**Symptom:** "permission denied" errors

**Solution:**
```bash
# Fix file permissions
sudo chown -R echo:echo /var/lib/echo
sudo chmod 755 /var/lib/echo

# Fix binary permissions
sudo chmod +x /usr/local/bin/echo
```

---

### Authentication Issues

#### Invalid Token

**Symptom:** 401 Unauthorized with "invalid token"

**Causes:**
1. Token expired
2. Token was revoked
3. JWT secret changed

**Solution:**
```bash
# Get new token
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}'

# Or use refresh token
curl -X POST http://localhost:8080/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token":"your-refresh-token"}'
```

#### API Key Not Working

**Symptom:** 401 with API key authentication

**Check:**
```bash
# Verify API key format
# Should be: ek_xxxxx...

# Check if key has required scopes
curl http://localhost:8080/api/v1/apikeys \
  -H "Authorization: Bearer <admin-token>"
```

#### RBAC Permission Denied

**Symptom:** 403 Forbidden

**Solution:**
```bash
# Check user's role and permissions
curl http://localhost:8080/api/v1/auth/me \
  -H "Authorization: Bearer <token>"

# Update user role (admin only)
# Or adjust RBAC config:
rbac:
  roles:
    - name: user
      permissions: ["chat", "skills.execute", "plugins.list"]
```

---

### LLM Connection Issues

#### Ollama Not Responding

**Symptom:** "connection refused" to Ollama

**Solution:**
```bash
# Check if Ollama is running
curl http://localhost:11434/api/tags

# Start Ollama
ollama serve

# Check if model is available
ollama list

# Pull model if missing
ollama pull llama2
```

#### OpenAI API Errors

**Symptom:** 401 or 429 from OpenAI

**Common causes:**
1. Invalid API key
2. Rate limit exceeded
3. Insufficient quota

**Solution:**
```bash
# Verify API key
curl https://api.openai.com/v1/models \
  -H "Authorization: Bearer $OPENAI_API_KEY"

# Check usage at https://platform.openai.com/usage
```

#### Anthropic API Errors

**Symptom:** Authentication or rate limit errors

**Solution:**
```bash
# Verify API key format (should start with sk-ant-)
echo $ANTHROPIC_API_KEY

# Test API key
curl https://api.anthropic.com/v1/messages \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d '{"model":"claude-3-opus-20240229","max_tokens":10,"messages":[{"role":"user","content":"Hi"}]}'
```

#### Slow LLM Responses

**Symptom:** Long response times

**Solutions:**
1. Use a smaller/faster model
2. Reduce max_tokens
3. Use local Ollama instead of cloud APIs
4. Check network latency

```yaml
llm:
  providers:
    openai:
      model: "gpt-3.5-turbo"  # Faster than gpt-4
      max_tokens: 500          # Reduce token limit
```

---

### Plugin Issues

#### Plugin Won't Load

**Symptom:** Plugin not appearing in list

**Check:**
```bash
# Verify plugin directory
ls -la ./plugins/

# Check plugin manifest
cat ./plugins/my-plugin/manifest.json

# Check logs for loading errors
grep "plugin" /var/log/echo/echo.log
```

**Common issues:**
1. Invalid manifest.json
2. Missing required fields
3. JavaScript syntax errors

#### Plugin Execution Errors

**Symptom:** Plugin fails during execution

**Debug:**
```bash
# Enable debug logging
logging:
  level: "debug"

# Check plugin-specific logs
grep "my-plugin" /var/log/echo/echo.log
```

#### JavaScript Plugin Timeout

**Symptom:** "execution timeout" error

**Solution:**
```yaml
plugins:
  execution_timeout: "30s"  # Increase timeout
```

---

### Database Issues

#### Database Locked

**Symptom:** "database is locked" error (SQLite)

**Solution:**
```bash
# Check for multiple processes
fuser /var/lib/echo/echo.db

# Increase busy timeout in config
database:
  busy_timeout: "10s"

# Or switch to a different database
database:
  type: "postgres"
  dsn: "postgres://user:pass@localhost/echo"
```

#### Database Corruption

**Symptom:** "database disk image is malformed"

**Solution:**
```bash
# Backup current database
cp /var/lib/echo/echo.db /var/lib/echo/echo.db.bak

# Try to recover
sqlite3 /var/lib/echo/echo.db ".recover" | sqlite3 /var/lib/echo/echo_new.db

# Or restore from backup
cp /var/lib/echo/backups/latest/echo.db /var/lib/echo/echo.db
```

---

### Performance Issues

#### High Memory Usage

**Symptom:** Memory consumption keeps growing

**Solutions:**
1. Limit concurrent connections
2. Reduce conversation history size
3. Enable memory limits

```yaml
server:
  max_connections: 100

chat:
  max_history_messages: 50
```

#### High CPU Usage

**Symptom:** CPU constantly at 100%

**Debug:**
```bash
# Enable profiling
profiling:
  enabled: true
  endpoint_prefix: "/debug/pprof"

# Get CPU profile
curl http://localhost:8080/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof
```

#### Slow API Responses

**Symptom:** API latency > 1s (excluding LLM time)

**Solutions:**
1. Enable response caching
2. Optimize database queries
3. Check disk I/O

```bash
# Check disk I/O
iostat -x 1

# Check database query times
logging:
  level: "debug"
  include_sql: true
```

---

### Network Issues

#### CORS Errors

**Symptom:** Browser console shows CORS errors

**Solution:**
```yaml
server:
  cors:
    enabled: true
    allowed_origins:
      - "http://localhost:3000"
      - "https://your-domain.com"
    allowed_methods:
      - "GET"
      - "POST"
      - "PUT"
      - "DELETE"
```

#### WebSocket Connection Failed

**Symptom:** WebSocket upgrade fails

**Check:**
1. Reverse proxy configuration
2. Firewall rules
3. WebSocket support enabled

**Nginx config:**
```nginx
location /api/v1/ws/ {
    proxy_pass http://localhost:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
}
```

#### SSL/TLS Issues

**Symptom:** Certificate errors

**Solution:**
```yaml
server:
  tls:
    enabled: true
    cert_file: "/etc/echo/cert.pem"
    key_file: "/etc/echo/key.pem"
```

Or use a reverse proxy (recommended):
```bash
# Let's Encrypt with certbot
sudo certbot --nginx -d echo.yourdomain.com
```

---

### Backup/Restore Issues

#### Backup Failed

**Symptom:** Backup job fails

**Check:**
```bash
# Verify backup directory exists and is writable
ls -la /var/lib/echo/backups/
mkdir -p /var/lib/echo/backups
chown echo:echo /var/lib/echo/backups

# Check disk space
df -h /var/lib/echo/
```

#### Restore Failed

**Symptom:** Cannot restore from backup

**Solution:**
```bash
# Stop service first
sudo systemctl stop echo

# Restore database
cp /var/lib/echo/backups/2024-01-15/echo.db /var/lib/echo/echo.db

# Restore config
cp /var/lib/echo/backups/2024-01-15/config.yaml /etc/echo/config.yaml

# Fix permissions
chown echo:echo /var/lib/echo/echo.db

# Start service
sudo systemctl start echo
```

---

## Diagnostic Commands

### System Information

```bash
# Echo version
echo --version

# Go version (if built from source)
go version

# System info
uname -a
cat /etc/os-release

# Memory
free -h

# Disk
df -h
```

### Network Diagnostics

```bash
# Check listening ports
sudo netstat -tlnp | grep echo

# Test connectivity
curl -v http://localhost:8080/health

# DNS resolution
nslookup api.openai.com
```

### Log Analysis

```bash
# Count errors in last hour
journalctl -u echo --since "1 hour ago" | grep -c "error"

# Find most common errors
journalctl -u echo --since "1 day ago" | grep "error" | sort | uniq -c | sort -rn | head

# Watch for specific issues
journalctl -u echo -f | grep -E "(error|panic|fatal)"
```

---

## Getting Help

If you can't resolve your issue:

1. **Check existing issues:** [GitHub Issues](https://github.com/IceWhaleTech/ZimaOS-Echo/issues)

2. **Collect diagnostic info:**
   ```bash
   echo --version
   cat /etc/echo/config.yaml
   journalctl -u echo --since "1 hour ago" > echo-logs.txt
   ```

3. **Open a new issue** with:
   - Echo version
   - OS and version
   - Steps to reproduce
   - Error messages
   - Relevant logs

4. **Community support:**
   - [Discord](https://discord.gg/zimaos)
   - [Forum](https://forum.zimaos.com)
