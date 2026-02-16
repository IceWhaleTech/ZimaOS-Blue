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
  <a href="https://discord.gg/SrCYvumF"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>&nbsp;&nbsp;
  <img src="../../docs/assets/wechat.png" height="128"/>
  <details style="display:inline-block;">
    <summary style="list-style:none;cursor:pointer;">
    </summary>

  </details>
</p>

## Введение

Вдохновлённые Clawdbot, мы верим, что **будущее** персональных вычислений будет **определяться разнообразными локальными ИИ-агентами**, работающими на периферии.

**ZimaOS Blue — наш ответ** — полностью **открытая, проверяемая и готовая к продакшену среда выполнения и набор инструментов для агентов**, позволяющая развёртывать приватных, самостоятельно размещённых агентов без лишних сложностей.

Создан для смелых разработчиков, которые хотят **экспериментировать или вручную создавать собственных агентов**. Blue **спроектирован для производительности**: написан на **Go**, с потреблением памяти от 10 МБ. Работает на **любом x86, Raspberry Pi, Windows, macOS** — везде, где есть питание.

![](../../docs/assets/features.png)

## Ключевые особенности

### Локальный подход и автоматический доступ к моделям

Идём дальше: нативная поддержка **20+ IM-платформ**, **голосовые** интерфейсы для естественного контекстного диалога, **автоматическое переключение моделей** со сканированием IDE и многоуровневые персональности SOUL.

### Быстрый и лёгкий

Нативная компиляция на Go — без интерпретатора, без виртуальной машины, без накладных расходов. Бесшумно работает на всём: от серверов до настольных устройств.

| Метрика | ZimaOS Blue (Go) | OpenClaw (Node + dist) |
|---------|-------------------|------------------------|
| `--help` холодный / горячий | **0.18 s / < 0.01 s** | 3.31 s / ~1.11 s |
| `status` время выполнения (лучший из 3) | **< 0.01 s** | 5.98 s |
| `--help` пиковое RSS | **~10 MB** | ~394 MB |
| `status` пиковое RSS | **~15 MB** | ~1.52 GB |
| Зависимости среды выполнения | **Нет** | Node.js 18+ |

> Тестирование на macOS arm64, один хост, лучший из 3 запусков. Февраль 2026.

### Чистый Go, любое устройство

100% Go, статический бинарник. **Кросс-компиляция для 5 платформ** из коробки (![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) amd64/arm64, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) amd64/arm64, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) amd64). Без Node, без Python, без контейнеров. Разместите на NAS, Raspberry Pi, старом x86-роутере или Mac — он просто работает. **Затем добавляйте свой UI, логику и навыки агента** — одна кодовая база, любая платформа.

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

Мы минимизируем шаблонный код, чтобы вы **сосредоточились на главном**. Следуя **философии дизайна ZimaOS**, Blue обеспечивает:
- **От нуля до результата в один клик** – Мгновенное развёртывание без сложной настройки.
- **Быстрое прототипирование** – Экспериментируйте или вручную создавайте инструменты, взаимодействия и пакеты приложений для конкретных сценариев.
- **Готовность к глобальному рынку** – **Мир огромен**, и он не говорит только по-английски. **20+ языков, нативно**, без барьеров.
- **Открытая экосистема моделей** – Без привязки к поставщику. Используйте свои модели.

![](../../docs/assets/design_principle.png)

## Быстрый старт

### Вариант 1: Скачать десктопное приложение (macOS и Windows)

Получите нативное приложение — без зависимостей, без компиляции.

- **macOS**: [Скачать DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- **Windows**: [Скачать установщик](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Вариант 2: Скрипт установки

**macOS / Linux**
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Вариант 3: Сборка из исходников

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
./dev.sh
```

## Обзор архитектуры

![](../../docs/assets/architecture.png)

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
  → (no match) → Chat Handler → LLM → Humanizer (MD→text) → Channel → User
```

**Голосовой конвейер**
```
WebSocket audio → STT (Whisper) → LLM Processing → TTS (eSpeak/Edge) → WebSocket audio
```

### Карта пакетов (`server/internal/`)

| Уровень | Пакеты |
|---------|--------|
| Шлюз | bootstrap, server, gateway |
| Прокси | proxy, connection, streaming, resilience |
| Провайдер | providerpool, providers, llm |
| Обрезка контекста | pruner (detector, segmenter, bm25, pipeline, cache) |
| Агент | context, tools, personality, humanizer |
| Память | memory, embedding, kvstore |
| Каналы | channel, autoreply, i18n |
| Безопасность | security, auth, permission, rbac, mfa, password, oidc, extauth, sandbox, promptguard, audit |
| Голос | voice, tts, stt, speech |
| Наблюдение | metrics, heartbeat, companion, profiling, leakdetect |
| Плагины | plugin, skill, skillstore |
| Интеграции | browser, homeassistant, cron, workflow, formfiller, tunnel, crawler |
| Планировщик | scheduler, worker, workerpool, pool |
| Ядро | lifecycle, config, logger, database, cache, ratelimit, retry, timeutil, sync |
| Система | sysinfo, cgroup, iotask, watcher, resources, backup, update |
| Мультитенантность | tenant, user, session, preview |

## Как использовать

![](../../docs/assets/handcraft.png)

## Хронология вех

![](../../docs/assets/timeline.png)

| Version | Фокус | Ключевая ценность | Status |
|---------|-------|--------------------|--------|
| v0.1 | Ядро среды выполнения Go | Стабильное ядро, работа 24 часа | Done |
| v0.2 | Базовые возможности | Минимально пригодный, интеграция с LLM | Done |
| v0.3 | Интеграция с NAS | Нативная поддержка NAS, systemd | Done |
| v0.4 | Система плагинов | Расширяемость, основы безопасности | Done |
| v0.5 | Продуктовый базис | Готовность к продакшену, документация | Done |
| v0.6 | Каналы сообщений | Многоканальная поддержка | Done |
| v0.7 | Безопасность | OIDC, MFA, аудит | Done |
| v0.8 | Производительность | Оптимизация, кэширование, бенчмарки | Done |
| v0.9 | Экосистема | Мультитенантность, автоматизация браузера, голос | Done |
| v0.10.0 | Интеграция CLI | Интеграция CC CLI, обнаружение, автообновление | Done |
| v0.10.1 | Мониторинг метрик | Статистика API, отслеживание токенов, TTFT | Done |
| v0.10.2 | Надёжность CLI | Жизненный цикл процессов, восстановление после ошибок | Done |
| v0.10.3 | Интеграция CLI | Мастер настройки, автоопределение провайдера | Done |
| v0.10.4 | Упаковка Tauri | Десктопное приложение, системный трей | Done |
| v0.10.5 | API Proxy Sidecar | Выбор маршрута, защита промптов, статистика использования | Done |
| v0.10.6 | Пул провайдеров | Маршрутизация между провайдерами, проверка здоровья, отказоустойчивость | Done |
| v0.10.7 | Режим предпросмотра | Неаутентифицированный доступ, управление функциями | Done |
| v0.10.8 | Магазин навыков | Инфраструктура магазина навыков, валидация каналов | Done |
| v0.10.9–10 | Управление пользователями | Подпользователи, разрешения на уровне страниц | Done |
| v0.10.13–14 | Безопасность и навыки | Страница безопасности, редизайн магазина навыков | Done |
| v0.10.15 | Улучшения чата | UX чата, конвейер сообщений | Done |
| v0.10.16 | Модуль речи | Sherpa TTS/ASR, eSpeak, переключение провайдеров | Done |
| v0.10.17 | Удалённый доступ | Туннели Ngrok, Cloudflare, сертификаты ACME | Done |
| v0.10.18–20 | Спринт производительности | Производительность запуска/чата, кэш контекста | Done |
| v0.10.21–22 | Промпты и DingTalk | Системный промпт, канал DingTalk | Done |
| v0.10.23 | OTA-обновления | Система OTA-обновлений | Done |
| v0.10.24 | Обновление каналов | 10 каналов обновлены из заглушек | Done |
| v0.10.25 | CC Cache | Двухуровневый кэш (L1 память + L2 диск) | Done |
| v0.10.26 | Гуманизатор | Конвейер гуманизации ответов | Done |
| v0.10.27 | Обрезка контекста | Оценка BM25, сегментация, бенчмарки | Done |
| v0.10.28 | Сервис памяти | Прогрессивный поиск, двойная запись | Done |

## Сообщество и поддержка

- **Проблемы**: [Сообщайте об ошибках и предлагайте функции здесь](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Обсуждения**: [Discord](https://discord.gg/SrCYvumF)
- **Следите за нами** на [GitHub](https://github.com/IceWhaleTech)

## Лицензия

Этот проект лицензирован под лицензией MIT — подробности в файле [LICENSE](../../LICENSE). Мы верим в открытый исходный код и вклад в сообщество.

## Участники

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
