![](../../docs/assets/bannerX.png)

<h2 align="center">ZimaOS Blue: Un runtime de agentes local-first para creadores audaces</h2>

<p align="center"><strong>Listo desde el primer momento · Código abierto · Universal · Neutral frente a proveedores</strong></p>

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
  <a href="https://discord.gg/b3AgFDxe9v"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>
</p>

## Introducción

Inspirándonos en OpenClaw, creemos que el futuro de la informática personal estará determinado por diversos agentes de IA locales que se ejecuten en el borde.

ZimaOS Blue es nuestra respuesta: un conjunto de herramientas y tiempo de ejecución de agente totalmente de código abierto, auditable, neutral respecto al proveedor y listo para producción que le permite enviar agentes privados y autohospedados sin fricciones.

Creado para desarrolladores audaces que desean darle vida o crear a sus propios agentes, Blue está diseñado para el rendimiento: escrito en Go, con una huella de memoria tan baja como 19 MB. Se ejecuta en cualquier x86, <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS, en cualquier lugar donde se conecte la alimentación.

## Demostraciones

### Conversación y ejecución de tareas

Una demostración rápida del flujo de conversación y la ejecución de tareas en Blue.

<p align="center">
  <img src="../../docs/assets/demo.gif" alt="Conversation & Task Execution demo" width="800" />
</p>

### Integración de proveedores LLM

Una demostración rápida de la experiencia de integración de proveedores LLM en Blue.

<p align="center">
  <img src="../../docs/assets/demo Provider.gif" alt="LLM Providers Integration demo" width="800" />
</p>

### Vista rápida - Vista general, canales y configuración adicional

Una demostración rápida que cubre la vista general del producto, los canales y la configuración adicional.

<p align="center">
  <img src="../../docs/assets/demo quickv4.gif" alt="Quick Overview demo" width="720" />
</p>

## Por qué Blue

<p align="center">
  <img src="../../docs/assets/design_principle.png" alt="Design Principle" />
</p>

### Pure Go, cualquier dispositivo

100% Go, binario estático. Compilación cruzada de 5 objetivos listos para usar (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) `linux/amd64`, `linux/arm64`, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) `darwin/amd64`, `darwin/arm64`, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) `windows/amd64`). No se requiere tiempo de ejecución de nodo, Python ni contenedores. Colóquelo en un NAS, <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS, un ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, un enrutador x86 antiguo o un ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac; simplemente se ejecuta. Luego, aplique su propia interfaz de usuario, lógica y habilidades de agente: una base de código, cada plataforma.

### Listo para usar, listo para trabajar

Todo el mundo quiere herramientas que sean simples, confiables y escalables cuando las necesite. Herramientas que simplemente funcionan, para que puedas concentrarte en lo que realmente estás creando.

Esta no es una filosofía nueva. Es el mismo que construyó <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS: simple, confiable y diseñado para mantenerse fuera de su camino. Blue es esa filosofía, extendida a la pila de agentes.

### Diseñado para tu vida, creado para permanecer local

Desde una investigación profunda que ofrece un informe HTML completo hasta OCR, PDF, automatización del navegador y conversión de documentos, Blue maneja flujos de trabajo complejos del mundo real sin enviar sus datos a la nube. La activación por voz, STT/TTS, Talk Mode y la compatibilidad con la inferencia local hacen que las interacciones cotidianas sean instantáneas, privadas y siempre disponibles.

## Inicio rápido

### Opción 1: Descargar la aplicación de escritorio

Obtenga la aplicación nativa: sin dependencias ni compilación. Configuración de prueba incorporada con incorporación en segundos: comience a chatear instantáneamente a través de una conexión remota, no se requiere configuración de bot. Verdadera experiencia lista para usar.

- <img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> **ZimaOS**: [Ejecutar en ZimaOS](https://www.zimaspace.com/zimaos?utm_source=blue)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [Descargar DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [Descargar instalador](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Opción 2: Instalar script

<img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> ZimaOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Opción 3: compilar desde la fuente

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
git submodule update --init --recursive
```

<img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="16" /> ZimaOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
sh build.sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
.\build.bat
```

> **Nota:** Las compilaciones de Windows requieren:
> - [MinGW-w64](https://www.mingw-w64.org/) (gcc) y [CMake](https://cmake.org/) para dependencias nativas de C (espeak-ng, susurro.cpp, opus, kokoro, onnx)
> - [SDK de Windows](https://developer.microsoft.com/en-us/windows/downloads/windows-sdk/) para bibliotecas del sistema (winmm, etc.)
>
> Asegúrese de que `gcc`, `cmake` estén en su `PATH`.

## Descripción general de la arquitectura

<p align="center">
  <img src="../../docs/assets/architecture.png" alt="architecture" />
</p>

Vaya más allá: ofrece soporte nativo para **más de 20 plataformas de mensajería instantánea**, **interfaces controladas por voz** para un diálogo natural y contextual, **cambio de modelo de configuración cero** con escaneo IDE.

<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>

## Cómo construir

<p align="center">
  <img src="../../docs/assets/handcraft.png" alt="handcraft" />
</p>

> ⚠️ [!IMPORTANT]
>
> Si planeas seguir sintonizando o codificando vibe además de Blue, no trates algunos chats atractivos como evidencia de publicación. Cualquier cambio que afecte el enrutamiento, el comportamiento de ejecución, la superficie de la herramienta, el control del presupuesto, la selección del modelo o el marco de ejecución debe validarse con Blue Harness, no con comprobaciones puntuales ad hoc.
>
> Blue debe seguir una regla simple aquí: los datos primero, las puertas primero, el corte al final. En la práctica, eso significa actualizar el Harness conjunto de datos/evaluación de especificación relevante antes de juzgar un cambio, luego mantener un `candidate_id` estable durante todo el intento para que los informes de selector, ejecución, presupuesto y preparación describan al mismo candidato en lugar de cuatro ejecuciones no relacionadas.

### Flujo de trabajo recomendado Harness

1. Ejecute `blue harness selector verify`
2. Ejecute `blue harness execution verify`
3. Reutilice la ejecución de evaluación del selector para `blue harness budget gate`
4. Termina con `blue harness cutover-readiness`

Para iteración local, validación nocturna o recopilación de evidencia de CI, prefiera `python3 scripts/cutover_candidate_pipeline.py`. Ejecuta el selector completo -> ejecución -> presupuesto -> secuencia de preparación bajo un candidato compartido, lo que hace que el resultado sea más fácil de comparar, revisar y eliminar.

### Barandillas adicionales

| Área | Qué mirar |
|------|----------------|
| Estabilidad inicial | Mantenga estables la línea de base, la versión del conjunto de datos y `candidate_id`, o la comparación se desviará y el resultado no será confiable. |
| Producción real de construcción | Reconstruya el paquete binario o frontend afectado antes de ejecutar Harness; de lo contrario, puede terminar validando un comportamiento obsoleto en lugar del cambio actual. |
| Registro de ruta | Si el frontend y el backend cambian juntos, confirme que todas las rutas de backend nuevas estén realmente registradas antes de juzgar la característica a través del comportamiento de la interfaz de usuario, porque el registro faltante a menudo parece un error lógico pero en realidad es un `404`. |
| Sentencia de liberación | Un pase de ajuste solo está listo cuando Harness no muestra una regresión significativa y la preparación para la transición confirma que el candidato está realmente listo para la transición. |

En resumen, sintonizar con Blue no se trata de "se siente mejor en unas cuantas charlas". Se trata de poner al candidato en Harness, recopilar evidencia comparable y dejar que los resultados de entrada y preparación decidan si es realmente seguro mantener el cambio.

## Características

| Característica | Qué ofrece |
|---------|-------------------|
| Recuperación web de alta disponibilidad y tiempo de ejecución del navegador | Uno de los **mejores diferenciadores** de Blue. Blue unifica **cuatro rutas de acceso web** para buscar, leer, extraer y rastrear; mantiene **tres capas alternativas** en HTTP, extracción de proxy y sesiones de navegador; maneja **páginas anti-bot** con detección de desafíos, reutilización de cookies/sesiones, sigilo y transferencia del navegador; y rutas a través de **tres motores de navegador**: `lightpanda`, Chromium administrado y Chrome local/de retransmisión. |
| Tiempo de ejecución de investigación tres en uno | **Una entrada de investigación pública** puede dirigirse a `deep_research`, `analyze` y `ui_review`. La misma pila de descubrimiento y evidencia produce luego **investigación basada en las citas**, **informes delimitados** y **revisiones estructuradas de UI/UX/accesibilidad**. |
| Harness Marco de ejecución, evaluación y evolución | Hace que la evaluación sea una **primitiva en tiempo de ejecución** en todo el desarrollo, la capacitación y la producción. Harness cubre **regresión y controles de humo**, puntuación, líneas de base, informes y validación del tiempo de ejecución, luego lleva la misma evidencia a **evolución de habilidades**, evaluación de seguimiento, promoción o reversión, y `AGENTS.md` o revisión de propuesta de instrucción. |
| Tiempo de ejecución multimodal con capacidad nativa primero | Mantiene **voz, OCR, PDF, tareas del navegador, conversión de documentos, llenado de formularios estructurados, procesamiento de medios y generación local de medios** en **rutas nativas y locales primero**, con **enrutamiento del modelo solo cuando es realmente necesario**. |
| Seguridad y Gobernanza | Incluye **ejecución de espacio aislado**, **defensa de inyección rápida**, **auditoría de sesión**, permisos, **RBAC**, **WebAuthn**, barreras de seguridad operativas y **escaneo de seguridad de habilidades**. |
| LLM Wiki y espacio de conocimiento | Convierte los resultados de la memoria, la investigación y el tiempo de ejecución en una **superficie de conocimiento similar a una wiki** con **páginas de resumen**, índices, **vínculos de retroceso**, **actualización** y **flujos de trabajo de archivo**. |
| Tienda de habilidades y mercado | Incluye **descubrimiento de habilidades**, selección, sincronización y **escaneo local** integrados, por lo que la extensibilidad está disponible **desde el primer día**. |
| Grupo de proveedores de nivel de producción | Proporciona un grupo de proveedores real con **verificaciones de estado**, **conmutación por error automática**, **disyuntores** y **carreras de proveedores** para cargas de trabajo de larga duración. |
| Tiempo de ejecución de modelo pequeño local integrado | Incluye un tiempo de ejecución integrado **`Qwen3.5-0.8B` + `llama.cpp`** para **preguntas y respuestas cortas locales**, reconocimiento de imágenes, enrutamiento de herramientas, resumen, **compresión de contexto** y **preprocesamiento de documentos**. |
| Fiabilidad a largo plazo | Trata las actualizaciones **OTA**, **copia de seguridad y restauración**, **recarga en caliente de configuración** y **recuperación posterior a un error** como **preocupaciones operativas integradas**. |

## Cronología de hitos

<p align="center">
  <img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</p>

| Fecha | Versión | Palabras clave/Características |
|------|---------|---------------------|
| 26 de enero de 2026 | `v0.1–v0.9` | Go runtime, sistema de complementos, automatización del navegador |
| 27 y 28 de enero de 2026 | `v0.9.0–v0.9.2` | Vista de tareas del navegador, Blue Companion, Smart Form Filler |
| 29 al 31 de enero de 2026 | `v0.10.0–v0.10.9` | Claude Code CLI, API Proxy, reestructuración de la interfaz de usuario |
| 1 al 3 de febrero de 2026 | `v0.10.1–v0.10.22` | Métricas, acceso remoto, caché de contexto |
| 5 al 18 de febrero de 2026 | `v0.10.25–v0.10.29` | i18n, CC Cache, canal de lanzamiento |
| 20 al 25 de febrero de 2026 | `v0.10.28–v0.10.29` | Cargador de escritorio, UX móvil, rediseño de memoria |
| 28 de febrero al 2 de marzo de 2026 | `v0.10.30` | Deep Research, reclasificador de habilidades, análisis de seguridad |
| 9 al 18 de marzo de 2026 | `v0.10.31` | Revisión del panel, refactor VoiceChat, sitios aprobados |
| 19 al 22 de marzo de 2026 | `v0.10.32` | Harness implementación, auditoría de expedientes académicos, búsqueda web |
| 23 al 25 de marzo de 2026 | `v0.10.33` | Grupos Harness, aprobaciones de navegadores, mercado de habilidades |
| 29 y 30 de marzo de 2026 | `v0.10.35` | Harness v3, retransmisión del navegador, compresión de contexto |
| 31 de marzo a 1 de abril de 2026 | `v0.10.36` | Auditoría de transcripciones, superposiciones Harness, análisis de herramientas |
| 1 de abril de 2026 | `v0.10.37` | Endurecimiento del tiempo de ejecución, transición de Skill+Exec, pulido de recuperación |
| 2 al 5 de abril de 2026 | `v0.10.38` | Soporte GitHub, refinamiento del mercado, mejoras en la confiabilidad |
| 6 y 7 de abril de 2026 | `v0.10.39` | Unificación de investigaciones, superficies de evolución, reducción de la huella de memoria |

## Comunidad y soporte

- **Problemas**: [Presente errores y solicitudes de funciones aquí](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Discusiones**: [Discord](https://discord.gg/zwWbKA4S2)
- **Síguenos** en [GitHub](https://github.com/IceWhaleTech)

[![Star History Chart](https://api.star-history.com/svg?repos=IceWhaleTech/ZimaOS-Blue&type=Date)](https://star-history.com/#IceWhaleTech/ZimaOS-Blue&Date)

## Licencia

Este proyecto tiene la licencia MIT; consulte el archivo [LICENCIA](../../LICENCIA) para obtener más detalles. Creemos en el código abierto y en retribuir a la comunidad.

## Colaboradores

Gracias a todos los Blue contribuyentes:

<a href="https://community.vaunt.dev/board/IceWhaleTech/repository/ZimaOS-Blue">
  <img src="https://api.vaunt.dev/v1/github/entities/IceWhaleTech/repositories/ZimaOS-Blue/contributors?format=svg&limit=30" width="450" />
</a>

## Referencias

1. **OpenClaw** — Agente de código abierto primero local. Fue pionero en conectar LLM a dispositivos locales a través de adaptadores de canal y llamadas de herramientas, inspirando directamente la arquitectura de tiempo de ejecución del agente de Blue. https://github.com/openclaw/openclaw
2. **MiroMind** — Modo de investigación profunda con síntesis respaldada por evidencia. Dio forma al profundo proceso de investigación integrado de Blue: planificación, recuperación paralela, deduplicación de evidencia y generación de informes HTML. https://www.miromind.ai
3. **Karpathy's LLM Wiki** — LLM como compilador de conocimientos. Replantea los LLM para construir espacios de conocimiento persistentes y en evolución, yendo más allá de la trampa de acumulación de RAG.
4. **OpenSpace (HKUDS)** — Motor de habilidades de evolución automática. Un marco basado en DAG donde los agentes aprenden de las fallas y obtienen habilidades especializadas. https://github.com/HKUDS/OpenSpace
5. **Andrew Ng's Context Hub** — Registro de documentación API versionada para agentes de codificación. Aborda las alucinaciones de los agentes y los conocimientos olvidados de la sesión. Proporciona documentos seleccionados y versionados con anotaciones y ciclos de retroalimentación, lo que convierte la documentación en una capa de conocimiento que se mejora a sí misma. https://github.com/andrewyng/context-hub
6. **Notion** — Sencillo, humano e intencionalmente silencioso. Inspirado en el espíritu minimalista de Notion, Blue devuelve calidez a la parrilla. Donde el serif refinado se combina con un diseño cuidadoso, creando un espacio que se siente como en casa. https://www.notion.com/about
7. **Matrix** — Inspiración visual de la icónica estética de la lluvia digital. La dirección estética de los esquemas técnicos de Blue.
8. **IceWhale** — Amor, Muerte y Robots T2E2 "Hielo". Un colectivo que se reúne en todo el mundo para romper los muros de los gigantes de Internet y resistir la concentración de datos. La ballena de hielo simboliza una comunidad que construye herramientas soberanas en el borde.
9. **ZimaOS Blue** — Amor, muerte y robots T1E14 "Zima Blue". Una metáfora: inteligencia que comienza en el servicio y evoluciona para explorar el mundo. Blue es un agente de sabiduría, arraigado en la simplicidad y buscando la profundidad.
10. **ZimaOS** — Principios de diseño abiertos, enfocados y simplificados. Tanto ZimaOS como Blue comparten la creencia de que la tecnología debe estar al servicio del usuario: implementarse en 30 segundos, ejecutarse en cualquier lugar y mantenerse neutral con respecto al proveedor. https://www.zimaspace.com/zimaos
