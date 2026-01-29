# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <a href="../README.md">English</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../en_GB/README.md">English (UK)</a> |
  <strong>Español (ES)</strong> |
  <a href="../fr_FR/README.md">Français</a> |
  <a href="../de_DE/README.md">Deutsch</a> |
  <a href="../it_IT/README.md">Italiano</a> |
  <a href="../pt_PT/README.md">Português (PT)</a> |
  <a href="../pt_BR/README.md">Português (BR)</a> |
  <a href="../ru_RU/README.md">Русский</a> |
  <a href="../ja_JP/README.md">日本語</a> |
  <a href="../ko_KR/README.md">한국어</a> |
  <a href="../nl_NL/README.md">Nederlands</a> |
  <a href="../sv_SE/README.md">Svenska</a> |
  <a href="../da_DK/README.md">Dansk</a> |
  <a href="../fi_FI/README.md">Suomi</a> |
  <a href="../no_NO/README.md">Norsk</a> |
  <a href="../pl_PL/README.md">Polski</a> |
  <a href="../tr_TR/README.md">Türkçe</a> |
  <a href="../cs_CZ/README.md">Čeština</a> |
  <a href="../el_GR/README.md">Ελληνικά</a> |
  <a href="../ca_ES/README.md">Català</a> |
  <a href="../ga_IE/README.md">Gaeilge</a> |
  <a href="../ar_SA/README.md">العربية</a> |
  <a href="../hi_IN/README.md">हिन्दी</a> |
  <a href="../th_TH/README.md">ไทย</a> |
  <a href="../id_ID/README.md">Bahasa Indonesia</a> |
  <a href="../vi_VN/README.md">Tiếng Việt</a>
</p>

> Esta es la versión en español (España) del README de ZimaOS Echo.

Para la documentación completa, visita: https://echo.zimaos.com  
El resto de este documento sigue la estructura del README principal en inglés. Consulta `../README.md` para la información más actualizada y detallada.

# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>Entorno de Ejecución de Agentes Nativo para NAS</strong>
</p>

<p align="center">
  <a href="../../README.md">English</a> | <a href="../zh_CN/README.md">中文</a> | <strong>Español</strong>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Echo/ci.yml?branch=main&style=for-the-badge" alt="Estado CI"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Echo/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Echo?include_prereleases&style=for-the-badge" alt="Versión GitHub"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="Licencia MIT"></a>
</p>

**ZimaOS Echo** es un entorno de ejecución de agentes de IA ligero y de alto rendimiento diseñado específicamente para dispositivos NAS y edge. Construido con Go, proporciona una plataforma lista para producción para ejecutar asistentes de IA en hardware de bajo consumo.

[Documentación](https://echo.zimaos.com) · [Inicio Rápido](#inicio-rápido) · [Características](#características) · [Comparación](#comparación-con-clawdbot)

## ¿Por qué ZimaOS Echo?

ZimaOS Echo está inspirado en [clawdbot](https://github.com/clawdbot/clawdbot) pero reconstruido desde cero en Go para:

- **Menor Uso de Recursos**: Funciona en dispositivos con solo 256MB de RAM
- **Mejor Rendimiento**: Binario nativo de Go con concurrencia eficiente basada en goroutines
- **Despliegue Más Fácil**: Un solo binario, no requiere Node.js
- **Optimización para NAS**: Diseñado para operación 24/7 en dispositivos de bajo consumo

## Inicio Rápido

### Linux / macOS

```bash
curl -fsSL https://echo.zimaos.com/install.sh | sudo bash
```

### Windows (PowerShell como Administrador)

```powershell
irm https://echo.zimaos.com/install.ps1 | iex
```

### Desde el Código Fuente

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo/server
go build -o zimaos-echo ./cmd/server
./zimaos-echo server
```

## Características

### Características Principales

- 🚀 **Ligero**: Un solo binario < 15MB, memoria < 80MB
- ⚡ **Alto Rendimiento**: Basado en Go con concurrencia de goroutines
- 🔌 **Multi-Proveedor**: OpenAI, Anthropic, Ollama y más
- 🛡️ **Listo para Producción**: Circuit breaker, degradación elegante, recuperación automática
- 📊 **Observable**: Métricas Prometheus, perfilado pprof, registro estructurado
- 🔄 **Recarga en Caliente**: Cambios de configuración sin reiniciar
- 💾 **Copia de Seguridad/Restauración**: Copia de seguridad automatizada con restauración a un punto en el tiempo

### Integración de Hogar Inteligente

- 🏠 **Home Assistant**: Integración nativa con la API de Home Assistant
- 💡 **Control de Dispositivos**: Luces, interruptores, sensores, clima y más
- 🤖 **Automatización IA**: Comandos en lenguaje natural para control del hogar inteligente
- 📡 **Eventos en Tiempo Real**: Suscripción a cambios de estado de dispositivos vía WebSocket

### Capacidades de Voz

- 🎤 **Reconocimiento de Voz**: Conversión de voz a texto basada en Whisper
- 🔊 **Texto a Voz**: Soporte para múltiples motores TTS
- 👂 **Activación por Voz**: Detección de palabra de activación personalizable
- 🗣️ **Comandos de Voz**: Interacción con el asistente de IA sin manos

### Arquitectura Multi-Inquilino

- 👥 **Aislamiento de Inquilinos**: Aislamiento completo de datos y recursos
- 🔐 **Autenticación por Inquilino**: Autenticación independiente por inquilino
- 📊 **Cuotas de Recursos**: Límites de CPU, memoria y tasa de API por inquilino
- 🎛️ **Panel de Inquilino**: Portal de gestión de autoservicio

### Canales de Comunicación

- 💬 **Protocolo Matrix**: Mensajería descentralizada y cifrada de extremo a extremo
- 📱 **Telegram/Discord/Slack**: Soporte para plataformas de mensajería populares
- 📞 **Signal/WhatsApp**: Integración de mensajería segura
- 🍎 **iMessage**: Soporte nativo de iMessage para macOS

### Seguridad y Autenticación

- 🔑 **WebAuthn/Passkeys**: Autenticación sin contraseña con FIDO2
- 🔐 **OIDC/OAuth 2.0**: SSO empresarial (Google, GitHub, Okta, etc.)
- 📲 **MFA/TOTP**: Soporte para autenticación multifactor
- 🛡️ **RBAC**: Control de acceso basado en roles detallado
- 📝 **Registro de Auditoría**: Rastro de auditoría de seguridad completo
- 🔒 **Sandbox**: Entorno de ejecución aislado para herramientas

### Frontend

- 🎨 **Panel Vue 3**: Interfaz web moderna y responsiva
- 💬 **Interfaz de Chat**: Respuestas en streaming con soporte Markdown
- 📈 **Monitor del Sistema**: Gráficos de uso de recursos en tiempo real
- ⚙️ **UI de Configuración**: Gestión de configuración fácil

## Comparación con Clawdbot

ZimaOS Echo está inspirado en clawdbot pero optimizado para despliegue NAS/edge:

| Característica | ZimaOS Echo | Clawdbot |
|----------------|-------------|----------|
| **Lenguaje** | Go | TypeScript/Node.js |
| **Tamaño del Binario** | ~15MB | ~200MB+ (con node_modules) |
| **Uso de Memoria** | ~80MB inactivo | ~200MB+ inactivo |
| **Tiempo de Inicio** | < 1s | 3-5s |
| **Entorno de Ejecución** | Binario nativo | Requiere Node.js |
| **Plataforma Objetivo** | Dispositivos NAS/Edge | Escritorio/Servidor |

## Arquitectura

```
┌─────────────────────────────────────────────────┐
│                  ZimaOS-Echo                     │
├─────────────────────────────────────────────────┤
│  Frontend Vue 3  │  REST API  │  WebSocket      │
├─────────────────────────────────────────────────┤
│              Entorno de Ejecución Principal (Go) │
│  Bucle de Eventos │ Pool de Trabajadores │ Config │ Logger │
├─────────────────────────────────────────────────┤
│              Entorno de Ejecución de Agentes     │
│  Proveedor LLM │ Herramientas │ Memoria │ Contexto │
├─────────────────────────────────────────────────┤
│              Capa de Datos                       │
│  SQLite (Zorm) │ ECache │ Archivos              │
└─────────────────────────────────────────────────┘
```

## Hoja de Ruta

- [x] **v0.1.0** - Entorno de Ejecución Principal (Bucle de eventos, Pool de trabajadores, Config, Logger)
- [x] **v0.2.0** - Entorno de Ejecución de Agentes (Proveedores LLM incl. AWS Bedrock, Herramientas, Memoria)
- [x] **v0.3.0** - Capa API (REST, WebSocket, Streaming)
- [x] **v0.4.0** - Sistema de Plugins (Módulos Go, soporte WASM)
- [x] **v0.5.0** - Listo para Producción (Métricas, Perfilado, Copia de seguridad)
- [x] **v0.6.0** - Canales de Mensajería (Telegram, Discord, Slack, WhatsApp, Signal, iMessage)
- [x] **v0.7.0** - Seguridad (OIDC, MFA, WebAuthn, Registro de auditoría, Sandbox)
- [x] **v0.8.0** - Rendimiento (ECache, Zorm, HTTP/2)
- [x] **v0.9.0** - Mejoras Futuras (A2UI, Automatización del Navegador)
- [ ] **v1.0.0** - RAG y Base de Conocimiento

## Contribuir

¡Las contribuciones son bienvenidas! Por favor, lee nuestra [Guía de Contribución](CONTRIBUTING.md) para más detalles.

```bash
# Clonar el repositorio
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo

# Instalar dependencias
cd server && go mod download

# Ejecutar pruebas
go test ./...

# Compilar
go build -o zimaos-echo ./cmd/server
```

## Licencia

Licencia MIT - ver [LICENSE](LICENSE) para más detalles.

## Agradecimientos

- [clawdbot](https://github.com/clawdbot/clawdbot) - Inspiración para el proyecto
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) - ORM ligero

---

<p align="center">
  Hecho con ❤️ por <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
