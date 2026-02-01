# ZimaOS Echo

<p align="center">
  <img src="../../docs/public/logo.png" alt="ZimaOS Echo" width="200">
</p>

<p align="center">
  <strong>Runtime de agente IA seguro e observável</strong>
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
  <strong>Português</strong> |
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

**ZimaOS Echo** é um runtime de agente IA leve e de alto desempenho projetado para NAS e dispositivos de borda. Desenvolvido em Go, oferece uma plataforma pronta para produção com implantação zero-config, monitoramento de sessões e análise de uso.

[Início rápido](#início-rápido) · [Recursos](#recursos-principais)

## Destaques

| Especificação | Valor |
|------|------|
| **Tamanho do binário** | ~40 MB (executável único) |
| **Memória (ocioso)** | ~4 MB |
| **Tempo de inicialização** | < 1 s |
| **Dependências** | Nenhuma (implantação zero-config) |

## Recursos principais

### Implantação zero-config

- **Binário único**: baixar e executar, sem dependências de runtime
- **Configuração sob demanda**: funciona imediatamente, personalizável quando necessário
- **Multiplataforma**: Windows, macOS, Linux – mesmo binário, mesma experiência
- **Suporte a daemon**: execução como serviço em segundo plano

### Monitoramento de sessões

- **Rastreamento em tempo real**: monitorar todas as sessões IA ativas e seu status
- **Histórico de conversas**: trilha de auditoria completa de todas as interações
- **Reprodução de sessões**: revisar e analisar conversas passadas
- **Isolamento multi-tenant**: separação completa de sessões entre usuários

### Otimização da cadeia de chamadas

- **Rastreamento de requisições**: visibilidade fim a fim de cada chamada de API
- **Análise de latência**: identificar gargalos no pipeline de requisições
- **Roteamento de provedores**: roteamento inteligente para provedores LLM ótimos
- **Circuit breaker**: failover automático em falhas de provedor

### Análise de uso

- **Consumo de tokens**: rastreamento por usuário, sessão e provedor
- **Atribuição de custos**: detalhamento de custos por operação
- **Limite de taxa**: gerenciamento de cota por tenant
- **Exportar relatórios**: gerar relatórios de uso em vários formatos

### Reforço de segurança

- **Execução em sandbox**: todas as chamadas de ferramentas em ambientes isolados
- **RBAC**: controle de acesso granular baseado em funções
- **WebAuthn/Passkeys**: autenticação FIDO2 sem senha
- **MFA/TOTP**: autenticação multifator
- **Trilha de auditoria**: logs imutáveis de todas as operações privilegiadas

## Início rápido

```bash
# A partir do código-fonte
git clone https://github.com/IceWhaleTech/ZimaOS-Echo.git
cd ZimaOS-Echo
make build && ./dist/zimaos-echo server
```

Acesse o painel em `http://localhost:3000`.

## Configuração de provedores LLM

ZimaOS Echo suporta vários provedores LLM, incluindo serviços LLM locais:

```yaml
llm:
  # Provedores em nuvem
  provider: "openai"  # ou "anthropic", "azure", etc.
  api_key: "your-api-key"

  # LLM local (opcional)
  # provider: "ollama"
  # base_url: "http://localhost:11434"
```

## Arquitetura

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

## Observabilidade

```yaml
# Habilitar pilha completa de observabilidade
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

### Métricas expostas

- Latência de requisições (p50, p95, p99)
- Uso de tokens LLM por provedor
- Taxas de sucesso/falha de execução de ferramentas
- Contagens de memória e goroutines
- Transições de estado do circuit breaker

## Ambiente de desenvolvimento

### Pré-requisitos

| Ferramenta | Versão | Instalação |
|------|---------|---------|
| Go | 1.21+ | [golang.org](https://golang.org/dl/) |
| Node.js | 18+ | [nodejs.org](https://nodejs.org/) |
| Make | - | Pré-instalado em macOS/Linux |

### Modo desenvolvimento (hot reload)

```bash
# Linux / macOS
./dev.sh

# Windows
dev.bat
```

- Frontend: `http://localhost:3000`
- Backend: `http://localhost:23456`

### Comandos de build

```bash
make build              # Binário único (frontend incorporado)
make build-embedded     # Build com Claude Code CLI incorporado
make build-all          # Compilação cruzada para todas as plataformas
make clean              # Limpar artefatos de build
```

### Estrutura do projeto

```
ZimaOS-Echo/
├── server/             # Backend Go
│   ├── cmd/echo/       # Ponto de entrada
│   └── internal/       # Módulos principais
├── web/                # Frontend Vue 3
│   └── src/
└── dist/               # Saída de build
```

## Agradecimentos

- [clawdbot](https://github.com/clawdbot/clawdbot) – Inspiração do projeto
- [IceWhaleTech/zorm](https://github.com/IceWhaleTech/zorm) – ORM leve

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
