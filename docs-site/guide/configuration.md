# Configuration

ZimaOS Echo uses YAML configuration files with environment variable overrides.

## Configuration File

Default location: `/opt/zimaos-echo/config/config.yaml`

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: "30s"
  write_timeout: "30s"
  idle_timeout: "120s"

log:
  level: "info"      # debug, info, warn, error
  format: "json"     # json, console
  output: "stdout"   # stdout, stderr, or file path

worker:
  pool_size: 10
  max_queue_len: 100
```

## Environment Variables

All configuration options can be overridden with environment variables using the `ECHO_` prefix:

| Config Key | Environment Variable | Default |
|------------|---------------------|---------|
| `server.host` | `ECHO_SERVER_HOST` | `0.0.0.0` |
| `server.port` | `ECHO_SERVER_PORT` | `8080` |
| `log.level` | `ECHO_LOG_LEVEL` | `info` |
| `log.format` | `ECHO_LOG_FORMAT` | `json` |
| `worker.pool_size` | `ECHO_WORKER_POOL_SIZE` | `10` |

Example:

```bash
export ECHO_SERVER_PORT=9090
export ECHO_LOG_LEVEL=debug
./echo --config config.yaml
```

## Server Configuration

### `server.host`

The IP address to bind to.

- `0.0.0.0` - Listen on all interfaces
- `127.0.0.1` - Listen only on localhost

### `server.port`

The port number to listen on. Default: `8080`

### `server.read_timeout`

Maximum duration for reading the entire request. Default: `30s`

### `server.write_timeout`

Maximum duration before timing out writes of the response. Default: `30s`

### `server.idle_timeout`

Maximum amount of time to wait for the next request. Default: `120s`

## Logging Configuration

### `log.level`

Log verbosity level:

- `debug` - Detailed debugging information
- `info` - General operational information
- `warn` - Warning messages
- `error` - Error messages only

### `log.format`

Output format:

- `json` - Structured JSON logs (recommended for production)
- `console` - Human-readable colored output

### `log.output`

Where to write logs:

- `stdout` - Standard output
- `stderr` - Standard error
- `/path/to/file.log` - Write to file

## Worker Configuration

### `worker.pool_size`

Number of concurrent workers in the pool. Default: `10`

Recommended values:
- Low-power NAS: `5-10`
- Standard server: `10-20`
- High-performance: `20-50`

### `worker.max_queue_len`

Maximum number of tasks waiting in queue. Default: `100`

## Command Line Options

```bash
./echo [OPTIONS]

Options:
  --config, -c    Path to configuration file
  --version, -v   Show version information
  --help, -h      Show help message
```

## Example Configurations

### Development

```yaml
server:
  host: "127.0.0.1"
  port: 8080

log:
  level: "debug"
  format: "console"
  output: "stdout"

worker:
  pool_size: 5
```

### Production

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: "60s"
  write_timeout: "60s"

log:
  level: "info"
  format: "json"
  output: "/var/log/zimaos-echo/echo.log"

worker:
  pool_size: 20
  max_queue_len: 200
```
