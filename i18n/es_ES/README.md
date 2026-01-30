# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>Runtime de agente IA seguro y observable</strong>
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

**ZimaOS Echo** es un runtime de agente IA ligero y de alto rendimiento diseñado para NAS y dispositivos edge. Desarrollado en Go, ofrece una plataforma lista para producción con despliegue sin configuración, supervisión de sesiones y análisis de uso.

[Inicio rápido](#inicio-rápido) · [Funciones](#funciones-principales)

## Destacados

| Especificación | Valor |
|------|------|
| **Tamaño del binario** | ~40 MB (ejecutable único) |
| **Memoria (inactivo)** | ~4 MB |
| **Tiempo de arranque** | < 1 s |
| **Dependencias** | Ninguna (despliegue sin configuración) |

## Funciones principales

### Despliegue sin configuración

- **Binario único**: descargar y ejecutar, sin dependencias de tiempo de ejecución
- **Configuración bajo demanda**: funciona de inmediato, personalizable cuando haga falta
- **Multiplataforma**: Windows, macOS, Linux – mismo binario, misma experiencia
- **Soporte demonio**: ejecución como servicio en segundo plano

### Supervisión de sesiones

- **Seguimiento en tiempo real**: supervisar todas las sesiones IA activas y su estado
- **Historial de conversaciones**: trazabilidad completa de todas las interacciones
- **Reproducción de sesiones**: revisar y analizar conversaciones pasadas
- **Aislamiento multi-tenant**: separación completa de sesiones entre usuarios

### Optimización de la cadena de llamadas

- **Trazado de peticiones**: visibilidad de extremo a extremo de cada llamada API
- **Análisis de latencia**: identificar cuellos de botella en el flujo de peticiones
- **Enrutado de proveedores**: enrutado inteligente a los proveedores LLM óptimos
- **Circuit breaker**: conmutación automática ante fallos de proveedor

### Análisis de uso

- **Consumo de tokens**: seguimiento por usuario, sesión y proveedor
- **Atribución de costes**: desglose detallado de costes por operación
- **Límite de tasa**: gestión de cuotas por tenant
- **Exportar informes**: generar informes de uso en varios formatos

### Refuerzo de seguridad

- **Ejecución en sandbox**: todas las llamadas a herramientas se ejecutan en entornos aislados
- **RBAC**: control de acceso basado en roles de grano fino
- **WebAuthn/Passkeys**: autenticación FIDO2 sin contraseña
- **MFA/TOTP**: autenticación multifactor
- **Registro de auditoría**: registros inmutables de todas las operaciones privilegiadas

## Inicio rápido

```bash
# Desde fuentes
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo
make build && ./dist/zimaos-echo server
```

Accede al panel en `http://localhost:3000`.

## Configuración de proveedores LLM

ZimaOS Echo admite varios proveedores LLM, incluidos servicios LLM locales:

```yaml
llm:
  # Proveedores en la nube
  provider: "openai"  # o "anthropic", "azure", etc.
  api_key: "your-api-key"

  # LLM local (opcional)
  # provider: "ollama"
  # base_url: "http://localhost:11434"
```

## Arquitectura

```
┌─────────────────────────────────────────────────────┐
│                    ZimaOS Echo                       │
├─────────────────────────────────────────────────────┤
│  Session Monitor │ Usage Analytics │ Call Tracing  │
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

## Observabilidad

```yaml
# Habilitar la pila de observabilidad completa
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

## Entorno de desarrollo

### Requisitos previos

| Herramienta | Versión | Instalación |
|------|---------|---------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| Make | - | Preinstalado en macOS/Linux |

### Modo desarrollo (recarga en caliente)

```bash
# Linux / macOS
./dev.sh

# Windows
dev.bat
```

- Frontend: `http://localhost:3000`
- Backend: `http://localhost:8080`

### Comandos de build

```bash
make build              # Binario único (frontend embebido)
make build-embedded     # Build con Claude Code CLI embebido
make build-all          # Compilación cruzada para todas las plataformas
make clean              # Limpiar artefactos de build
```

### Estructura del proyecto

```
ZimaOS-Echo/
├── server/             # Backend Go
│   ├── cmd/echo/       # Punto de entrada
│   └── internal/       # Módulos principales
├── web/                # Frontend Vue 3
│   └── src/
└── dist/               # Salida de build
```

## Agradecimientos

- [clawdbot](https://github.com/clawdbot/clawdbot) – Inspiración del proyecto
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) – ORM ligero

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
