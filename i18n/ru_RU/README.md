![](../../docs/assets/bannerX.png)

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="../ca_ES/README.md">Català</a> |
  <a href="../cs_CZ/README.md">Čeština</a> |
  <a href="../da_DK/README.md">Dansk</a> |
  <a href="../de_DE/README.md">Deutsch</a> |
  <a href="../el_GR/README.md">Ελληνικά</a> |
  <a href="../en_GB/README.md">English (UK)</a> |
  <a href="../es_ES/README.md">Español</a> |
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
  <strong>Русский</strong> |
  <a href="../sk_SK/README.md">Slovenčina</a> |
  <a href="../sv_SE/README.md">Svenska</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../zh_TW/README.md">繁體中文</a>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT License"></a>
</p>

<p align="center">
  <a href="https://discord.gg/b3AgFDxe9v"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>
</p>

## Введение

Вдохновлённые Clawdbot, мы верим, что **будущее** персональных вычислений будет **определяться разнообразными локальными ИИ-агентами**, работающими на периферии.

**ZimaOS Blue — наш ответ** — полностью **открытая, проверяемая и готовая к продакшену среда выполнения и набор инструментов для агентов**, позволяющая развёртывать приватных, самостоятельно размещённых агентов без лишних сложностей.

Создан для смелых разработчиков, которые хотят **экспериментировать или вручную создавать собственных агентов**. Blue **спроектирован для производительности**: написан на **Go**, с потреблением памяти от 10 МБ. Работает на **любом x86, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS** — везде, где есть питание.

![](../../docs/assets/features.png)

## Ключевые особенности

### Локальный подход и автоматический доступ к моделям

Идём дальше: нативная поддержка **20+ IM-платформ**, **голосовые** интерфейсы для естественного контекстного диалога, **автоматическое переключение моделей** со сканированием IDE и многоуровневые персональности SOUL.

<p align="center">
  <img src="../../docs/assets/channels.png" alt="Supported Channels" />
</p>

### Быстрый и лёгкий

Нативная компиляция на Go — без интерпретатора, без виртуальной машины, без накладных расходов. Бесшумно работает на всём: от серверов до настольных устройств.

| Метрика | ZimaOS Blue (Go) | OpenClaw (Node + dist) |
|---------|-------------------|------------------------|
| `--help` холодный / горячий | **0.18 s / < 0.01 s** | 3.31 s / ~1.11 s |
| `status` время выполнения (лучший из 3) | **< 0.01 s** | 5.98 s |
| `--help` пиковое RSS | **~10 MB** | ~394 MB |
| `status` пиковое RSS | **~15 MB** | ~1.52 GB |
| Зависимости среды выполнения | **Нет** | Node.js 18+ |

> Тестирование на macOS arm64 (серверный режим, без десктопного UI), один хост, лучший из 3 запусков. Февраль 2026.

### Чистый Go, любое устройство

100% Go, статический бинарник. **Кросс-компиляция для 5 платформ** из коробки (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) amd64/arm64, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) amd64/arm64, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) amd64). Без Node, без Python, без контейнеров. Разместите на NAS, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, старом x86-роутере или ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac — он просто работает. **Затем добавляйте свой UI, логику и навыки агента** — одна кодовая база, любая платформа.

### Безопасность и управление

Встроенный sidecar API-прокси с эшелонированной защитой:
- **Изолированное выполнение** – Все вызовы инструментов выполняются в изолированных средах.
- **Защита от инъекций промптов** – 7+ встроенных стратегий перехвата.
- **Аудит сессий** – Полный мониторинг сессий, каждое взаимодействие отслеживается.
- **RBAC и WebAuthn** – Детальный контроль доступа с беспарольной аутентификацией.

## Почему Blue

Мы верим, что **персональные вычисления нового поколения** используют LLM — но **контролируемые, проверяемые** агенты остаются основой как для отдельных пользователей, так и для команд. **Blue обеспечивает**:
- **Комплексное ядро** – Продвинутое управление моделями, интеграция с мессенджерами, расширенные персоны и интерфейсы на естественном языке, настроенные для повседневного использования (гарнитуры, голос, умные очки).
- **Локальный, сверхлёгкий, кроссплатформенный** – Не требует мощного оборудования. Работает на всём, что способно вычислять.
- **Безопасный и проверяемый** – Аудит сессий, песочница, контроль разрешений и встроенный API-прокси, действующий как межсетевой экран прикладного уровня — каждый байт на входе/выходе виден.

![](../../docs/assets/design_principle.png)

Мы минимизируем шаблонный код, чтобы вы **сосредоточились на главном**. Следуя <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> **философии дизайна ZimaOS**, Blue обеспечивает:
- **От нуля до результата в один клик** – Мгновенное развёртывание без сложной настройки.
- **Быстрое прототипирование** – Экспериментируйте или вручную создавайте инструменты, взаимодействия и пакеты приложений для конкретных сценариев.
- **Готовность к глобальному рынку** – **Мир огромен**, и он не говорит только по-английски. **20+ языков, нативно**, без барьеров.
- **Открытая экосистема моделей** – Без привязки к поставщику. Используйте свои модели.

<details>
<summary>
<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>
</summary>

| Провайдер | Модели | Тип |
|-----------|--------|-----|
| OpenAI | GPT-4o, GPT-4, o1, o3 | Облако |
| Anthropic | Claude 4.5, Claude 4 | Облако |
| Google | Gemini 2.5, Gemini 2.0 | Облако |
| Ollama | Llama, Qwen, Gemma, Phi и др. | Локально |
| DeepSeek | DeepSeek-V3, DeepSeek-R1 | Облако |
| Grok | Grok-3, Grok-3-mini | Облако |
| Qwen | Qwen-Max, Qwen-Plus, Qwen-Turbo | Облако |
| GLM | GLM-4, GLM-4-Flash | Облако |
| Moonshot | Moonshot-v1 | Облако |
| MiniMax | abab6.5, abab5.5 | Облако |
| Venice | Llama, Mistral (приватность) | Облако |
| AWS Bedrock | Claude, Llama, Titan | Облако |
| Azure | Модели OpenAI через Azure | Облако |
| OpenRouter | 100+ агрегированных моделей | Облако |
| AIHubMix | Мульти-провайдер агрегатор | Облако |
| Codex | OpenAI Codex | Облако |
| SiliconFlow | DeepSeek, Qwen, Llama via SiliconFlow | Облако |
| Пользовательский | Любой OpenAI / Anthropic / Gemini совместимый API | Облако / Локально |

</details>

### Поддерживаемые IDE

<p align="center">
  <img src="../../docs/assets/ides.png" alt="Supported IDEs" />
</p>

## Быстрый старт

### Вариант 1: Скачать десктопное приложение

Получите нативное приложение — без зависимостей, без компиляции. Встроенная пробная конфигурация, мгновенный старт — начните общение через удалённое подключение, без настройки бота. Настоящий «из коробки».

- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [Скачать DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [Скачать установщик](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Вариант 2: Скрипт установки

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Вариант 3: Сборка из исходников

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
```

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
sh build.sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
.\build.bat
```

## Обзор архитектуры

<details>
<summary>
<img src="../../docs/assets/architecture.png" alt="Architecture" />
</summary>

### Карта пакетов (`server/internal/`)

| Уровень | Пакеты |
|---------|--------|
| Gateway | bootstrap, server, gateway |
| Proxy | proxy, connection, streaming, resilience |
| Provider | providerpool, providers, llm |
| Pruner | pruner (detector, segmenter, bm25, pipeline, cache) |
| Agent | context, tools, personality, humanizer |
| Memory | memory, embedding, kvstore |
| Channel | channel, autoreply, i18n |
| Security | security, auth, permission, rbac, mfa, password, oidc, extauth, sandbox, promptguard, audit |
| Voice | voice, tts, stt, speech |
| Observe | metrics, heartbeat, companion, profiling, leakdetect |
| Plugin | plugin, skill, skillstore |
| Integrate | browser, homeassistant, cron, workflow, formfiller, tunnel, crawler |
| Scheduler | scheduler, worker, workerpool, pool |
| Core | lifecycle, config, logger, database, cache, ratelimit, retry, timeutil, sync |
| System | sysinfo, cgroup, iotask, watcher, resources, backup, update |
| Multi-tenant | tenant, user, session, preview |

</details>

### Поток данных

**Запрос чата (горячий путь прокси)**
```
Client [Proxy API Key] → Auth Gate → Prompt Guard → Context Pruner (optional)
  → Provider Pool (route:auto/cloud/local) → CC Cache (L1→L2) check
  → Upstream LLM → Response → Cache Store → Metrics Writer → Client (SSE stream)
```

**Поток сообщений каналов**
```
Telegram/Discord/... → Channel Manager → AutoReply check
  → Chat Handler → LLM → Humanizer (MD→text) → Channel → User
```

**Голосовой конвейер**
```
WebSocket audio → STT (Whisper) → LLM Processing → TTS (eSpeak/Edge) → WebSocket audio
```

**Мониторинг Heartbeat**
```
Таймер (30 мин) → Чтение HEARTBEAT.md → Оценка LLM → Удаление токена HEARTBEAT_OK
  → Дедупликация (хеш FNV, TTL 24ч) → Оповещение канала (Telegram/Slack/...)
  → Поток событий → Индикатор UI
```

## Как использовать

![](../../docs/assets/handcraft.png)

## Хронология вех

<details>
<summary>
<img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</summary>

| Версия | Фокус | Ключевая ценность | Статус |
|--------|-------|--------------------|--------|
| v0.1 | Ядро среды выполнения Go | Стабильное ядро, работа 24 часа | Готово |
| v0.2 | Базовые возможности | Минимально пригодный, интеграция с LLM | Готово |
| v0.3 | Интеграция с NAS | Нативная поддержка NAS, systemd | Готово |
| v0.4 | Система плагинов | Расширяемость, основы безопасности | Готово |
| v0.5 | Продуктовый базис | Готовность к продакшену, документация | Готово |
| v0.6 | Каналы сообщений | Многоканальная поддержка | Готово |
| v0.7 | Безопасность | OIDC, MFA, аудит | Готово |
| v0.8 | Производительность | Оптимизация, кэширование, бенчмарки | Готово |
| v0.9 | Экосистема | Мультитенантность, автоматизация браузера, голос | Готово |
| v0.10.0 | Интеграция CLI | Интеграция CC CLI, обнаружение, автообновление | Готово |
| v0.10.1 | Мониторинг метрик | Статистика API, отслеживание токенов, TTFT | Готово |
| v0.10.2 | Надёжность CLI | Жизненный цикл процессов, восстановление после ошибок | Готово |
| v0.10.3 | Интеграция CLI | Мастер настройки, автоопределение провайдера | Готово |
| v0.10.4 | Упаковка Tauri | Десктопное приложение, системный трей | Готово |
| v0.10.5 | API Proxy Sidecar | Выбор маршрута, защита промптов, статистика использования | Готово |
| v0.10.6 | Пул провайдеров | Маршрутизация между провайдерами, проверка здоровья, отказоустойчивость | Готово |
| v0.10.7 | Режим предпросмотра | Неаутентифицированный доступ, управление функциями | Готово |
| v0.10.8 | Магазин навыков | Инфраструктура магазина навыков, валидация каналов | Готово |
| v0.10.9–10 | Управление пользователями | Подпользователи, разрешения на уровне страниц | Готово |
| v0.10.13–14 | Безопасность и навыки | Страница безопасности, редизайн магазина навыков | Готово |
| v0.10.15 | Улучшения чата | UX чата, конвейер сообщений | Готово |
| v0.10.16 | Модуль речи | Sherpa TTS/ASR, eSpeak, переключение провайдеров | Готово |
| v0.10.17 | Удалённый доступ | Туннели Ngrok, Cloudflare, сертификаты ACME | Готово |
| v0.10.18–20 | Спринт производительности | Производительность запуска/чата, кэш контекста | Готово |
| v0.10.21–22 | Промпты и DingTalk | Системный промпт, канал DingTalk | Готово |
| v0.10.23 | OTA-обновления | Система OTA-обновлений | Готово |
| v0.10.24 | Обновление каналов | 10 каналов обновлены из заглушек | Готово |
| v0.10.25 | CC Cache | Двухуровневый кэш (L1 память + L2 диск) | Готово |
| v0.10.26 | Гуманизатор | Конвейер гуманизации ответов | Готово |
| v0.10.27 | Обрезка контекста | 54% экономия токенов на коде (SWE-bench официально), 46–47% на общих документах (локальный IR), оценка BM25, сегментация | Готово |
| v0.10.28 | Сервис памяти | Прогрессивный поиск, двойная запись | Готово |

</details>

## Сообщество и поддержка

- **Проблемы**: [Сообщайте об ошибках и предлагайте функции здесь](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Обсуждения**: [Discord](https://discord.gg/b3AgFDxe9v)
- **Следите за нами** на [GitHub](https://github.com/IceWhaleTech)

## Лицензия

Этот проект лицензирован под лицензией MIT — подробности в файле [LICENSE](../../LICENSE). Мы верим в открытый исходный код и вклад в сообщество.

## Участники

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
