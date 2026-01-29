# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>Runtime de agentes IA seguro, observable y local-first</strong>
</p>

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../zh_TW/README.md">繁體中文</a> |
  <a href="../ja_JP/README.md">日本語</a> |
  <a href="../ko_KR/README.md">한국어</a> |
  <a href="../de_DE/README.md">Deutsch</a> |
  <a href="../fr_FR/README.md">Français</a> |
  <strong>Español</strong> |
  <a href="../it_IT/README.md">Italiano</a> |
  <a href="../pt_BR/README.md">Português</a> |
  <a href="../ru_RU/README.md">Русский</a> |
  <a href="../ar_SA/README.md">العربية</a> |
  <a href="../hi_IN/README.md">हिन्दी</a> |
  <a href="../th_TH/README.md">ไทย</a> |
  <a href="../vi_VN/README.md">Tiếng Việt</a> |
  <a href="../id_ID/README.md">Bahasa Indonesia</a> |
  <a href="../tr_TR/README.md">Türkçe</a> |
  <a href="../pl_PL/README.md">Polski</a> |
  <a href="../nl_NL/README.md">Nederlands</a> |
  <a href="../sv_SE/README.md">Svenska</a>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Echo/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Echo?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT License"></a>
</p>

**ZimaOS Echo** es un runtime de agentes IA endurecido para NAS y dispositivos edge. Tus datos se quedan en tu hardware, cada acción es auditable y la IA se ejecuta en sandboxes aisladas.

[Documentación](https://echo.zimaos.com) · [Inicio rápido](#inicio-rápido) · [Funciones](#principios-clave) · [Comparación](#comparación-con-clawdbot)

## ¿Por qué ZimaOS Echo?

ZimaOS Echo se inspira en [clawdbot](https://github.com/clawdbot/clawdbot) y se reconstruye en Go para:

- **Menor uso de recursos**: corre en dispositivos con solo 256MB RAM
- **Mejor rendimiento**: binario Go nativo, concurrencia con gorutinas
- **Despliegue más fácil**: binario único, sin Node.js
- **Optimizado para NAS**: pensado para 24/7 en dispositivos de bajo consumo

## Principios clave

### Local-first

- **Soberanía de datos**: todo almacenado localmente en tu NAS, sin dependencia de la nube
- **Integración Ollama**: LLMs completamente en dispositivo, cero llamadas API externas
- **Funciona offline**: las funciones principales sin conexión
- **Binario único**: ~15 MB en Go nativo, sin dependencias de runtime

### Observable y auditable

- **Audit logging**: cada acción de IA registrada con contexto y marcas de tiempo
- **Métricas Prometheus**: supervisión en tiempo real de las operaciones
- **Perfilado pprof**: visibilidad de CPU, memoria y gorutinas
- **Logs estructurados**: JSON para parseo y alertas

### Endurecimiento de seguridad

- **Ejecución en sandbox**: todas las llamadas a herramientas en entornos aislados
- **RBAC**: control de acceso fino por roles
- **WebAuthn/Passkeys**: autenticación sin contraseña FIDO2
- **MFA/TOTP**: autenticación multifactor
- **OIDC/OAuth 2.0**: SSO empresarial
- **Circuit breaker**: aislamiento automático ante fallos, evita cascadas

## Inicio rápido

```bash
# Linux / macOS
curl -fsSL https://echo.zimaos.com/install.sh | sudo bash

# Desde fuentes
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo/server
go build -o zimaos-echo ./cmd/server
./zimaos-echo server
```

## Endurecimiento de seguridad

### Stack de autenticación

| Capa | Tecnología | Propósito |
|------|------------|-----------|
| Primaria | WebAuthn/Passkeys | Auth sin contraseña resistente al phishing |
| Secundaria | TOTP/MFA | Contraseñas de un solo uso temporales |
| Empresa | OIDC/OAuth 2.0 | SSO con Google, GitHub, Okta |
| Autorización | RBAC | Control de permisos por recurso |

### Protección en runtime

- **Aislamiento sandbox**: herramientas en entorno restringido
- **Rate limiting**: limitación de API por tenant
- **Aislamiento de tenants**: datos y recursos totalmente separados
- **Audit trail**: logs inmutables de operaciones privilegiadas

### Resiliencia

- **Circuit breaker**: aislamiento automático del servicio ante fallos
- **Degradación elegante**: estrategias de respaldo si falla un proveedor
- **Cadena de fallback LLM**: cambio automático de proveedor
- **Hot reload**: cambios de configuración sin reiniciar

## Observabilidad

```yaml
# Activar stack completo de observabilidad
metrics:
  enabled: true
  endpoint: "/metrics"

profiling:
  enabled: true
  endpoint_prefix: "/debug/pprof"

audit:
  enabled: true
  retention_days: 90
```

### Métricas expuestas

- Latencia de peticiones (p50, p95, p99)
- Uso de tokens LLM por proveedor
- Tasas de éxito/fallo de ejecución de herramientas
- Conteos de memoria y gorutinas
- Transiciones de estado del circuit breaker

## Arquitectura

```
┌─────────────────────────────────────────────────────┐
│                    ZimaOS Echo                       │
├─────────────────────────────────────────────────────┤
│  Audit Log  │  Metrics  │  RBAC  │  Rate Limiter   │
├─────────────────────────────────────────────────────┤
│              Sandbox Execution Layer                 │
│         Tool Isolation │ Resource Limits            │
├─────────────────────────────────────────────────────┤
│              Agent Runtime (Go)                      │
│  LLM Provider │ Tools │ Memory │ Circuit Breaker   │
├─────────────────────────────────────────────────────┤
│              Local Data Layer                        │
│  SQLite │ ECache │ Encrypted Storage                │
└─────────────────────────────────────────────────────┘
```

## Configuración de LLM local (Ollama)

Ejecutar IA totalmente offline, sin llamadas API externas:

```bash
# Instalar Ollama
curl -fsSL https://ollama.com/install.sh | sh

# Descargar modelo
ollama pull llama3.2

# Configurar Echo para LLM local
cat >> config.yaml << EOF
llm:
  provider: "ollama"
  model: "llama3.2"
  base_url: "http://localhost:11434"
EOF
```

## Entorno de desarrollo

### Requisitos

| Herramienta | Versión | Instalación |
|-------------|---------|-------------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| Make | - | Incluido en macOS/Linux |

### Arranque en un comando

```bash
# Clonar y arrancar
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo
make build && ./dist/zimaos-echo
```

Dashboard en `http://localhost:3000`

### Modo desarrollo (hot reload)

```bash
# Linux / macOS
./dev.sh

# Windows
dev.bat
```

- Frontend: `http://localhost:5173` (API proxy al backend)
- Backend: `http://localhost:8080`

### Comandos de build

```bash
make build              # Binario único (frontend embebido)
make build-embedded     # Build con Claude Code CLI embebido
make build-all          # Cross-compilar para todas las plataformas
make clean              # Limpiar artefactos de build
```

### Estructura del proyecto

```
ZimaOS-Echo/
├── server/             # Backend Go
│   ├── cmd/echo/       # Punto de entrada
│   └── internal/       # Módulos núcleo
├── web/                # Frontend Vue 3
│   └── src/
└── dist/               # Salida de build
```

## Comparación con Clawdbot

ZimaOS Echo está inspirado en clawdbot pero optimizado para NAS/edge:

| Concepto | ZimaOS Echo | Clawdbot |
|----------|-------------|----------|
| **Lenguaje** | Go | TypeScript/Node.js |
| **Tamaño binario** | ~15 MB | ~200 MB+ (con node_modules) |
| **Memoria** | ~80 MB inactivo | ~200 MB+ inactivo |
| **Arranque** | < 1 s | 3–5 s |
| **Runtime** | Binario nativo | Node.js requerido |
| **Plataforma** | NAS/Edge | Escritorio/Servidor |

## Agradecimientos

- [clawdbot](https://github.com/clawdbot/clawdbot) - Inspiración del proyecto
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) - ORM ligero

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
