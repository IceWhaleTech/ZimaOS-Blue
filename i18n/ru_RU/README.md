# ZimaOS Blue

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Blue" width="200">
</p>

<p align="center">
  <strong>Безопасная и наблюдаемая среда выполнения ИИ-агентов</strong>
</p>

<p align="center">
  <a href="../../README.md">English</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../zh_TW/README.md">繁體中文</a> |
  <a href="../ja_JP/README.md">日本語</a> |
  <a href="../ko_KR/README.md">한국어</a> |
  <a href="../de_DE/README.md">Deutsch</a> |
  <a href="../fr_FR/README.md">Français</a> |
  <a href="../es_ES/README.md">Español</a> |
  <a href="../it_IT/README.md">Italiano</a> |
  <a href="../pt_BR/README.md">Português</a> |
  <strong>Русский</strong> |
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
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="MIT License"></a>
</p>

**ZimaOS Blue** — лёгкая высокопроизводительная среда выполнения ИИ-агентов для NAS и периферийных устройств. Написана на Go, предоставляет готовую к продакшену платформу с развёртыванием без настройки, мониторингом сессий и аналитикой использования.

[Быстрый старт](#быстрый-старт) · [Возможности](#основные-возможности)

## Основное

| Параметр | Значение |
|------|------|
| **Размер бинарника** | ~40 МБ (один исполняемый файл) |
| **Память (простой)** | ~4 МБ |
| **Время запуска** | < 1 с |
| **Зависимости** | Нет (развёртывание без настройки) |

## Основные возможности

### Развёртывание без настройки

- **Один бинарник**: скачать и запустить, без зависимостей среды выполнения
- **Настройка по требованию**: работает из коробки, настраивается при необходимости
- **Кроссплатформенность**: Windows, macOS, Linux — один бинарник, один опыт
- **Поддержка демона**: запуск как фоновой службы

### Мониторинг сессий

- **Отслеживание в реальном времени**: мониторинг всех активных ИИ-сессий и их состояния
- **История диалогов**: полный аудит всех взаимодействий
- **Воспроизведение сессий**: просмотр и анализ прошлых диалогов
- **Мультитенантная изоляция**: полное разделение сессий между пользователями

### Оптимизация цепочки вызовов

- **Трассировка запросов**: сквозная видимость каждого API-вызова
- **Анализ задержек**: выявление узких мест в цепочке запросов
- **Маршрутизация провайдеров**: интеллектуальная маршрутизация к оптимальным LLM-провайдерам
- **Circuit Breaker**: автоматическое переключение при сбоях провайдера

### Аналитика использования

- **Расход токенов**: учёт по пользователю, сессии и провайдеру
- **Распределение затрат**: детальная разбивка по операциям
- **Ограничение частоты**: управление квотами по тенантам
- **Экспорт отчётов**: формирование отчётов об использовании в разных форматах

### Усиление безопасности

- **Песочница**: все вызовы инструментов выполняются в изолированной среде
- **RBAC**: детализированный контроль доступа по ролям
- **WebAuthn/Passkeys**: аутентификация FIDO2 без пароля
- **MFA/TOTP**: многофакторная аутентификация
- **Аудит**: неизменяемые журналы всех привилегированных операций

## Быстрый старт

```bash
# Из исходников
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
make build && ./dist/zimaos-blue server
```

Панель доступна по адресу `http://localhost:3000`.

## Настройка LLM-провайдеров

ZimaOS Blue поддерживает несколько LLM-провайдеров, включая локальные LLM-сервисы:

```yaml
llm:
  # Облачные провайдеры
  provider: "openai"  # или "anthropic", "azure" и т.д.
  api_key: "your-api-key"

  # Локальный LLM (опционально)
  # provider: "ollama"
  # base_url: "http://localhost:11434"
```

## Архитектура

```
┌─────────────────────────────────────────────────────┐
│                    ZimaOS Blue                       │
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

## Наблюдаемость

```yaml
# Включить полный стек наблюдаемости
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

### Доступные метрики

- Задержка запросов (p50, p95, p99)
- Использование токенов LLM по провайдерам
- Доля успешных/неуспешных выполнений инструментов
- Счётчики памяти и горутин
- Переходы состояний circuit breaker

## Среда разработки

### Требования

| Инструмент | Версия | Установка |
|------|---------|---------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| Make | - | Предустановлен в macOS/Linux |

### Режим разработки (горячая перезагрузка)

```bash
# Linux / macOS
./dev.sh

# Windows
dev.bat
```

- Фронтенд: `http://localhost:3000`
- Бэкенд: `http://localhost:23456`

### Команды сборки

```bash
make build              # Один бинарник (фронтенд встроен)
make build-embedded     # Сборка со встроенным Claude Code CLI
make build-all          # Кросс-компиляция для всех платформ
make clean              # Очистка артефактов сборки
```

### Структура проекта

```
ZimaOS-Blue/
├── server/             # Go-бэкенд
│   ├── cmd/blue/       # Точка входа
│   └── internal/       # Основные модули
├── web/                # Vue 3 фронтенд
│   └── src/
└── dist/               # Результат сборки
```

## Благодарности

- [clawdbot](https://github.com/clawdbot/clawdbot) — источник вдохновения проекта
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) — лёгкий ORM

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
