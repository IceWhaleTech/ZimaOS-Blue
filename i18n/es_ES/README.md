![](../../docs/assets/bannerX.png)

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="../ca_ES/README.md">Català</a> |
  <a href="../cs_CZ/README.md">Čeština</a> |
  <a href="../da_DK/README.md">Dansk</a> |
  <a href="../de_DE/README.md">Deutsch</a> |
  <a href="../el_GR/README.md">Ελληνικά</a> |
  <a href="../en_GB/README.md">English (UK)</a> |
  <strong>Español</strong> |
  <a href="../fr_FR/README.md">Français</a> |
  <a href="../ga_IE/README.md">Gaeilge</a> |
  <a href="../hr_HR/README.md">Hrvatski</a> |
  <a href="../hu_HU/README.md">Magyar</a> |
  <a href="../it_IT/README.md">Italiano</a> |
  <a href="../ja_JP/README.md">日本語</a> |
  <a href="../ko_KR/README.md">한국어</a> |
  <a href="../ml_IN/README.md">മലയാളം</a> |
  <a href="../nb_NO/README.md">Norsk Bokmål</a> |
  <a href="../nl_NL/README.md">Nederlands</a> |
  <a href="../pl_PL/README.md">Polski</a> |
  <a href="../pt_BR/README.md">Português (BR)</a> |
  <a href="../pt_PT/README.md">Português (PT)</a> |
  <a href="../ro_RO/README.md">Română</a> |
  <a href="../ru_RU/README.md">Русский</a> |
  <a href="../sk_SK/README.md">Slovenčina</a> |
  <a href="../sv_SE/README.md">Svenska</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../zh_TW/README.md">繁體中文</a>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="Estado CI"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="Versión en GitHub"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="Licencia MIT"></a>
</p>

<p align="center">
  <a href="https://discord.gg/SrCYvumF"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>&nbsp;&nbsp;
  <img src="../../docs/assets/wechat.png" height="128"/>
</p>

## Introducción

Inspirados por Clawdbot, creemos que el **futuro** de la computación personal será **moldeado por agentes de IA diversos y locales** ejecutándose en el borde.

**ZimaOS Blue es nuestra respuesta** — un **entorno de ejecución y kit de herramientas para agentes completamente de código abierto, auditable y listo para producción** que te permite desplegar agentes privados y autoalojados sin fricción alguna.

Construido para desarrolladores audaces que quieren **crear sus propios agentes con vibe coding o a mano**, Blue está **diseñado para el rendimiento**: escrito en **Go**, con un consumo de memoria tan bajo como 10 MB. Se ejecuta en **cualquier x86, Raspberry Pi, Windows, macOS** — donde sea que conectes la corriente.

![](../../docs/assets/features.png)

## Aspectos Destacados

### Diseño Local-First y Acceso Automático a Modelos

Lleva las cosas más lejos: ofrece soporte nativo para **más de 20 plataformas de mensajería instantánea**, interfaces **controladas por voz** para diálogos naturales y conscientes del contexto, **cambio de modelo sin configuración** con escaneo de IDE, y personalidades con capas SOUL.

### Rápido y Ligero

Compilado nativamente en Go — sin intérprete, sin VM, sin sobrecarga. Se ejecuta silenciosamente en todo, desde servidores hasta tus dispositivos de escritorio.

| Métrica | ZimaOS Blue (Go) | OpenClaw (Node + dist) |
|---------|-------------------|------------------------|
| `--help` frío / caliente | **0.18 s / < 0.01 s** | 3.31 s / ~1.11 s |
| `status` tiempo de ejecución (mejor de 3) | **< 0.01 s** | 5.98 s |
| `--help` RSS pico | **~10 MB** | ~394 MB |
| `status` RSS pico | **~15 MB** | ~1.52 GB |
| Dependencias de ejecución | **Ninguna** | Node.js 18+ |

> Pruebas realizadas en macOS arm64, mismo equipo, mejor de 3 ejecuciones. Feb 2026.

### Go Puro, Cualquier Dispositivo

100% Go, binario estático. **Compila cruzadamente para 5 objetivos** de forma nativa (![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) amd64/arm64, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) amd64/arm64, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) amd64). Sin runtime de Node, sin Python, sin contenedores necesarios. Colócalo en un NAS, una Raspberry Pi, un viejo router x86 o un Mac — simplemente funciona. **Luego añade tu propia interfaz, lógica y habilidades de agente** — un solo código base, todas las plataformas.

### Seguridad y Gobernanza

Proxy API sidecar integrado con defensa en profundidad:
- **Ejecución en Sandbox** – Todas las llamadas a herramientas se ejecutan en entornos aislados.
- **Defensa contra Inyección de Prompts** – Más de 7 estrategias de intercepción integradas.
- **Auditoría de Sesiones** – Monitoreo completo de sesiones, cada interacción es rastreable.
- **RBAC y WebAuthn** – Control de acceso granular con autenticación sin contraseña.

## Por Qué Blue

Creemos que la **computación personal de próxima generación** abraza los LLMs — pero los agentes **controlables y auditables** siguen siendo la base tanto para individuos como para equipos. **Blue ofrece**:
- **Núcleo Integral** – Gestión avanzada de modelos, integración con mensajería instantánea, personalidad mejorada e interfaces de lenguaje natural optimizadas para interacciones diarias (auriculares, voz, gafas inteligentes).
- **Local-First, Ultra Ligero, Multi-Dispositivo** – No requiere hardware de gama alta. Se ejecuta en cualquier cosa que pueda computar.
- **Seguro y Auditable** – Auditoría de sesiones, sandboxing, controles de permisos y un proxy API integrado que actúa como firewall a nivel de aplicación — cada byte de entrada/salida es visible.

Minimizamos el código repetitivo para que te **concentres en lo que importa**. Fiel a la **filosofía de diseño de ZimaOS**, Blue ofrece:
- **De Cero a Uno en Un Clic** – Despliega al instante, sin configuración compleja.
- **Prototipado Rápido** – Crea herramientas, interacciones y paquetes de aplicaciones específicos para cada escenario con vibe coding o a mano.
- **Listo para el Mundo** – **El mundo es enorme**, y no tiene por qué ser solo en inglés. **Más de 20 idiomas, nativos**, sin barreras.
- **Ecosistema de Modelos Abierto** – Sin dependencia de proveedores. Trae tus propios modelos.

![](../../docs/assets/design_principle.png)

## Inicio Rápido

### Opción 1: Descargar la Aplicación de Escritorio (macOS y Windows)

Obtén la aplicación nativa — sin dependencias, sin compilación.

- **macOS**: [Descargar DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- **Windows**: [Descargar Instalador](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Opción 2: Script de Instalación

**macOS / Linux**
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Opción 3: Compilar desde el Código Fuente

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
./dev.sh
```

## Visión General de la Arquitectura

![](../../docs/assets/architecture.png)

### Flujo de Datos

**Solicitud de Chat (Ruta Crítica del Proxy)**
```
Client [Proxy API Key] → Auth Gate → Prompt Guard → Context Pruner (optional)
  → Provider Pool (route:auto/cloud/local) → CC Cache (L1→L2) check
  → Upstream LLM → Response → Cache Store → Metrics Writer → Client (SSE stream)
```

**Flujo de Mensajes de Canal**
```
Telegram/Discord/... → Channel Manager → AutoReply check
  → (no match) → Chat Handler → LLM → Humanizer (MD→text) → Channel → User
```

**Pipeline de Voz**
```
WebSocket audio → STT (Whisper) → LLM Processing → TTS (eSpeak/Edge) → WebSocket audio
```

### Mapa de Paquetes (`server/internal/`)

| Capa | Paquetes |
|------|----------|
| Puerta de Enlace | bootstrap, server, gateway |
| Proxy | proxy, connection, streaming, resilience |
| Proveedor | providerpool, providers, llm |
| Podador | pruner (detector, segmenter, bm25, pipeline, cache) |
| Agente | context, tools, personality, humanizer |
| Memoria | memory, embedding, kvstore |
| Canal | channel, autoreply, i18n |
| Seguridad | security, auth, permission, rbac, mfa, password, oidc, extauth, sandbox, promptguard, audit |
| Voz | voice, tts, stt, speech |
| Observabilidad | metrics, heartbeat, companion, profiling, leakdetect |
| Plugin | plugin, skill, skillstore |
| Integración | browser, homeassistant, cron, workflow, formfiller, tunnel, crawler |
| Planificador | scheduler, worker, workerpool, pool |
| Núcleo | lifecycle, config, logger, database, cache, ratelimit, retry, timeutil, sync |
| Sistema | sysinfo, cgroup, iotask, watcher, resources, backup, update |
| Multi-inquilino | tenant, user, session, preview |

## Cómo Usar

![](../../docs/assets/handcraft.png)

## Cronograma de Hitos

![](../../docs/assets/timeline.png)

| Versión | Enfoque | Valor Clave | Estado |
|---------|---------|-------------|--------|
| v0.1 | Núcleo del Runtime en Go | Kernel estable, ejecución 24h | Done |
| v0.2 | Capacidades Básicas | Mínimo utilizable, integración LLM | Done |
| v0.3 | Integración NAS | NAS nativo, soporte systemd | Done |
| v0.4 | Sistema de Plugins | Extensible, bases de seguridad | Done |
| v0.5 | Línea Base del Producto | Listo para producción, documentación | Done |
| v0.6 | Canales de Mensajería | Soporte multicanal | Done |
| v0.7 | Seguridad | OIDC, MFA, auditoría | Done |
| v0.8 | Rendimiento | Optimización, caché, benchmarks | Done |
| v0.9 | Ecosistema | Multi-inquilino, automatización de navegador, voz | Done |
| v0.10.0 | Empaquetado CLI | Empaquetado CC CLI, detección, auto-actualización | Done |
| v0.10.1 | Monitoreo de Métricas | Estadísticas API, seguimiento de tokens, TTFT | Done |
| v0.10.2 | Fiabilidad CLI | Ciclo de vida de procesos, recuperación de errores | Done |
| v0.10.3 | Integración CLI | Asistente de configuración, auto-detección de proveedores | Done |
| v0.10.4 | Empaquetado Tauri | Aplicación de escritorio, bandeja del sistema | Done |
| v0.10.5 | Proxy API Sidecar | Selección de rutas, protección de prompts, estadísticas de uso | Done |
| v0.10.6 | Pool de Proveedores | Enrutamiento multi-proveedor, verificación de salud, failover | Done |
| v0.10.7 | Modo Vista Previa | Acceso sin autenticación, control de funcionalidades | Done |
| v0.10.8 | Tienda de Habilidades | Infraestructura de tienda de habilidades, validación de canales | Done |
| v0.10.9–10 | Gestión de Usuarios | Sub-usuarios, permisos a nivel de página | Done |
| v0.10.13–14 | Seguridad y Habilidades | Página de seguridad, rediseño de tienda de habilidades | Done |
| v0.10.15 | Mejoras de Chat | UX de chat, pipeline de mensajes | Done |
| v0.10.16 | Módulo de Voz | Sherpa TTS/ASR, eSpeak, cambio de proveedor | Done |
| v0.10.17 | Acceso Remoto | Túneles Ngrok, Cloudflare, certificados ACME | Done |
| v0.10.18–20 | Sprint de Rendimiento | Rendimiento de inicio/chat, caché de contexto | Done |
| v0.10.21–22 | Prompt y DingTalk | Prompt del sistema, canal DingTalk | Done |
| v0.10.23 | Actualización OTA | Sistema de actualización OTA | Done |
| v0.10.24 | Mejora de Canales | 10 canales actualizados desde stubs | Done |
| v0.10.25 | CC Cache | Caché de dos niveles (L1 memoria + L2 disco) | Done |
| v0.10.26 | Humanizador | Pipeline de humanización de respuestas | Done |
| v0.10.27 | Podador de Contexto | Puntuación BM25, segmentación, benchmarks | Done |
| v0.10.28 | Servicio de Memoria | Búsqueda progresiva, backend de escritura dual | Done |

## Comunidad y Soporte

- **Problemas**: [Por favor reporta errores y solicitudes de funcionalidades aquí](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Discusiones**: [Discord](https://discord.gg/SrCYvumF)
- **Síguenos** en [GitHub](https://github.com/IceWhaleTech)

## Licencia

Este proyecto está licenciado bajo la Licencia MIT - consulta el archivo [LICENSE](../../LICENSE) para más detalles. Creemos en el código abierto y en contribuir a la comunidad.

## Colaboradores

<p align="center">
  Hecho con ❤️ por <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
