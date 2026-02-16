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
  <strong>Português (PT)</strong> |
  <a href="../ro_RO/README.md">Română</a> |
  <a href="../ru_RU/README.md">Русский</a> |
  <a href="../sk_SK/README.md">Slovenčina</a> |
  <a href="../sv_SE/README.md">Svenska</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../zh_TW/README.md">繁體中文</a>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="Estado do CI"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="Versão no GitHub"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="Licença MIT"></a>
</p>

<p align="center">
  <a href="https://discord.gg/SrCYvumF"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>&nbsp;&nbsp;
  <img src="../../docs/assets/wechat.png" height="128"/>
</p>

## Introdução

Inspirados pelo Clawdbot, acreditamos que o **futuro** da computação pessoal será **moldado por agentes de IA diversos e local-first** a funcionar na periferia da rede.

**O ZimaOS Blue é a nossa resposta** — um **runtime e toolkit de agentes totalmente open-source, auditável e pronto para produção** que permite implementar agentes privados e auto-alojados sem qualquer fricção.

Concebido para programadores audazes que querem **criar os seus próprios agentes por vibe ou de forma artesanal**, o Blue é **projetado para desempenho**: escrito em **Go**, com um consumo de memória a partir de apenas 10 MB. Funciona em **qualquer x86, Raspberry Pi, Windows, macOS** — em qualquer lugar onde haja energia.

![](../../docs/assets/features.png)

## Destaques

### Design Local-First e Acesso Automático a Modelos

Indo mais longe: oferece suporte nativo a **mais de 20 plataformas de mensagens**, interfaces **orientadas por voz** para diálogos naturais e contextuais, **troca de modelos sem configuração** com varrimento de IDE, e personalidades em camadas SOUL.

### Rápido e Leve

Compilado nativamente em Go — sem interpretador, sem VM, sem overhead. Funciona silenciosamente em tudo, desde servidores aos seus dispositivos de secretária.

| Métrica | ZimaOS Blue (Go) | OpenClaw (Node + dist) |
|---------|-------------------|------------------------|
| `--help` frio / quente | **0.18 s / < 0.01 s** | 3.31 s / ~1.11 s |
| `status` tempo de execução (melhor de 3) | **< 0.01 s** | 5.98 s |
| `--help` pico de RSS | **~10 MB** | ~394 MB |
| `status` pico de RSS | **~15 MB** | ~1.52 GB |
| Dependências de runtime | **Nenhuma** | Node.js 18+ |

> Benchmark realizado em macOS arm64, mesmo anfitrião, melhor de 3 execuções. Fev 2026.

### Go Puro, Qualquer Dispositivo

100% Go, binário estático. **Compila de forma cruzada para 5 alvos** nativamente (![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) amd64/arm64, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) amd64/arm64, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) amd64). Sem runtime Node, sem Python, sem contentores. Coloque-o num NAS, num Raspberry Pi, num router x86 antigo ou num Mac — simplesmente funciona. **Depois acrescente a sua própria UI, lógica e competências de agente** — um único código-fonte, todas as plataformas.

### Segurança e Governação

Proxy de API sidecar integrado com defesa em profundidade:
- **Execução em Sandbox** – Todas as chamadas de ferramentas correm em ambientes isolados.
- **Defesa contra Injeção de Prompt** – Mais de 7 estratégias de interceção integradas.
- **Auditoria de Sessão** – Monitorização completa de sessão, cada interação rastreável.
- **RBAC e WebAuthn** – Controlo de acesso granular com autenticação sem palavra-passe.

## Porquê o Blue

Acreditamos que a **próxima geração de computação pessoal** abraça os LLMs — mas agentes **controláveis e auditáveis** continuam a ser a base tanto para indivíduos como para equipas. **O Blue oferece**:
- **Núcleo Abrangente** – Gestão avançada de modelos, integração com mensageiros, persona aprimorada e interfaces de linguagem natural otimizadas para interações diárias (auriculares, voz, óculos inteligentes).
- **Local-First, Ultra-Leve, Multi-Dispositivo** – Não requer hardware topo de gama. Funciona em qualquer coisa que compute.
- **Seguro e Auditável** – Auditoria de sessão, sandboxing, controlos de permissão e um proxy de API integrado que atua como firewall de camada de aplicação — cada byte de entrada/saída é visível.

Minimizamos o boilerplate para que se **foque no que importa**. Fiel à **filosofia de design do ZimaOS**, o Blue oferece:
- **Do Zero ao Um com Um Clique** – Implemente instantaneamente, sem configuração complexa.
- **Prototipagem Rápida** – Crie ferramentas, interações e pacotes de aplicações por vibe ou de forma artesanal.
- **Pronto para o Mundo** – **O mundo é enorme**, e não fala apenas inglês. **Mais de 20 idiomas, nativos**, sem barreiras.
- **Ecossistema Aberto de Modelos** – Sem dependência de fornecedor. Traga os seus próprios modelos.

![](../../docs/assets/design_principle.png)

## Início Rápido

### Opção 1: Transferir a Aplicação de Secretária (macOS e Windows)

Obtenha a aplicação nativa — sem dependências, sem compilação.

- **macOS**: [Transferir DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- **Windows**: [Transferir Instalador](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Opção 2: Script de Instalação

**macOS / Linux**
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Opção 3: Compilar a partir do Código-Fonte

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
./dev.sh
```

## Visão Geral da Arquitetura

![](../../docs/assets/architecture.png)

### Fluxo de Dados

**Pedido de Chat (Hot Path do Proxy)**
```
Client [Proxy API Key] → Auth Gate → Prompt Guard → Context Pruner (optional)
  → Provider Pool (route:auto/cloud/local) → CC Cache (L1→L2) check
  → Upstream LLM → Response → Cache Store → Metrics Writer → Client (SSE stream)
```

**Fluxo de Mensagens do Canal**
```
Telegram/Discord/... → Channel Manager → AutoReply check
  → (no match) → Chat Handler → LLM → Humanizer (MD→text) → Channel → User
```

**Pipeline de Voz**
```
WebSocket audio → STT (Whisper) → LLM Processing → TTS (eSpeak/Edge) → WebSocket audio
```

### Mapa de Pacotes (`server/internal/`)

| Camada | Pacotes |
|--------|---------|
| Gateway | bootstrap, server, gateway |
| Proxy | proxy, connection, streaming, resilience |
| Fornecedor | providerpool, providers, llm |
| Podador | pruner (detector, segmenter, bm25, pipeline, cache) |
| Agente | context, tools, personality, humanizer |
| Memória | memory, embedding, kvstore |
| Canal | channel, autoreply, i18n |
| Segurança | security, auth, permission, rbac, mfa, password, oidc, extauth, sandbox, promptguard, audit |
| Voz | voice, tts, stt, speech |
| Observabilidade | metrics, heartbeat, companion, profiling, leakdetect |
| Plugin | plugin, skill, skillstore |
| Integração | browser, homeassistant, cron, workflow, formfiller, tunnel, crawler |
| Agendador | scheduler, worker, workerpool, pool |
| Núcleo | lifecycle, config, logger, database, cache, ratelimit, retry, timeutil, sync |
| Sistema | sysinfo, cgroup, iotask, watcher, resources, backup, update |
| Multi-tenant | tenant, user, session, preview |

## Como Utilizar

![](../../docs/assets/handcraft.png)

## Cronograma de Marcos

![](../../docs/assets/timeline.png)

| Versão | Foco | Valor Principal | Estado |
|--------|------|-----------------|--------|
| v0.1 | Núcleo do Runtime Go | Kernel estável, execução 24h | Done |
| v0.2 | Capacidades Essenciais | Mínimo utilizável, integração com LLM | Done |
| v0.3 | Integração com NAS | NAS nativo, suporte a systemd | Done |
| v0.4 | Sistema de Plugins | Extensível, fundamentos de segurança | Done |
| v0.5 | Linha de Base do Produto | Pronto para produção, documentação | Done |
| v0.6 | Canais de Mensagem | Suporte multicanal | Done |
| v0.7 | Segurança | OIDC, MFA, auditoria | Done |
| v0.8 | Desempenho | Otimização, cache, benchmarks | Done |
| v0.9 | Ecossistema | Multi-tenant, automação de navegador, voz | Done |
| v0.10.0 | Empacotamento CLI | Empacotamento CC CLI, deteção, atualização automática | Done |
| v0.10.1 | Monitorização de Métricas | Estatísticas de API, rastreamento de tokens, TTFT | Done |
| v0.10.2 | Fiabilidade CLI | Ciclo de vida de processos, recuperação de erros | Done |
| v0.10.3 | Integração CLI | Assistente de configuração, deteção automática de fornecedor | Done |
| v0.10.4 | Empacotamento Tauri | Aplicação de secretária, tabuleiro do sistema | Done |
| v0.10.5 | Proxy de API Sidecar | Seleção de rota, proteção de prompt, estatísticas de utilização | Done |
| v0.10.6 | Pool de Fornecedores | Encaminhamento multi-fornecedor, health check, failover | Done |
| v0.10.7 | Modo Preview | Acesso não autenticado, controlo de funcionalidades | Done |
| v0.10.8 | Loja de Skills | Infraestrutura da loja de skills, validação de canais | Done |
| v0.10.9–10 | Gestão de Utilizadores | Sub-utilizadores, permissões por página | Done |
| v0.10.13–14 | Segurança e Skills | Página de segurança, redesign da loja de skills | Done |
| v0.10.15 | Melhorias no Chat | UX do chat, pipeline de mensagens | Done |
| v0.10.16 | Módulo de Fala | Sherpa TTS/ASR, eSpeak, troca de fornecedor | Done |
| v0.10.17 | Acesso Remoto | Túneis Ngrok, Cloudflare, certificados ACME | Done |
| v0.10.18–20 | Sprint de Desempenho | Desempenho de arranque/chat, cache de contexto | Done |
| v0.10.21–22 | Prompt e DingTalk | Prompt do sistema, canal DingTalk | Done |
| v0.10.23 | Atualização OTA | Sistema de atualização OTA | Done |
| v0.10.24 | Atualização de Canais | 10 canais atualizados de stubs | Done |
| v0.10.25 | CC Cache | Cache de dois níveis (L1 memória + L2 disco) | Done |
| v0.10.26 | Humanizer | Pipeline de humanização de respostas | Done |
| v0.10.27 | Context Pruner | Pontuação BM25, segmentação, benchmarks | Done |
| v0.10.28 | Serviço de Memória | Pesquisa progressiva, backend de escrita dupla | Done |

## Comunidade e Suporte

- **Issues**: [Registe bugs e pedidos de funcionalidades aqui](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Discussões**: [Discord](https://discord.gg/SrCYvumF)
- **Siga-nos** no [GitHub](https://github.com/IceWhaleTech)

## Licença

Este projeto está licenciado sob a Licença MIT - consulte o ficheiro [LICENSE](../../LICENSE) para mais detalhes. Acreditamos no open source e em retribuir à comunidade.

## Contribuidores

<p align="center">
  Feito com ❤️ por <a href="https://github.com/IceWhaleTech">IceWhaleTech</a>
</p>
