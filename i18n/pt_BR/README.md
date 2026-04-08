![](../../docs/assets/bannerX.png)

<h2 align="center">ZimaOS Blue: Um runtime de agentes local-first para builders ousados</h2>

<p align="center"><strong>Pronto para usar · Código aberto · Universal · Neutro em relação a fornecedores</strong></p>

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
  <strong>Português (BR)</strong> |
  <a href="../pt_PT/README.md">Português (PT)</a> |
  <a href="../ro_RO/README.md">Română</a> |
  <a href="../ru_RU/README.md">Русский</a> |
  <a href="../sk_SK/README.md">Slovenčina</a> |
  <a href="../sv_SE/README.md">Svenska</a> |
  <a href="../zh_CN/README.md">简体中文</a> |
  <a href="../zh_TW/README.md">繁體中文</a>
</p>

<p align="center">
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/actions"><img src="https://img.shields.io/github/actions/workflow/status/IceWhaleTech/ZimaOS-Blue/ci.yml?branch=main&style=for-the-badge" alt="Status do CI"></a>
  <a href="https://github.com/IceWhaleTech/ZimaOS-Blue/releases"><img src="https://img.shields.io/github/v/release/IceWhaleTech/ZimaOS-Blue?include_prereleases&style=for-the-badge" alt="Release no GitHub"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge" alt="Licença MIT"></a>
</p>

<p align="center">
  <a href="https://discord.gg/b3AgFDxe9v"><img src="../../docs/assets/discord.png" alt="Discord" height="128" /></a>&nbsp;&nbsp;
  <a href="https://www.facebook.com/zimaboard/"><img src="../../docs/assets/facebook.png" alt="Facebook" height="128" /></a>&nbsp;&nbsp;
  <a href="https://x.com/ZimaSpace"><img src="../../docs/assets/x.png" alt="X" height="128" /></a>
</p>

## Introdução

Inspirados pelo Clawdbot, acreditamos que o futuro da computação pessoal será moldado por diversos agentes de IA locais, operando na borda.

ZimaOS Blue é a nossa resposta: um kit de ferramentas e tempo de execução de agente totalmente aberto, auditável, neutro em termos de fornecedor e pronto para produção, que permite enviar agentes privados e auto-hospedados sem atrito.

Criado para desenvolvedores ousados ​​que desejam criar ou criar seus próprios agentes, o Blue foi projetado para desempenho: escrito em Go, com consumo de memória de apenas 19 MB. Ele roda em qualquer x86, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows, ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS – em qualquer lugar onde você conecte a energia.

## Demos

### Conversa e execução de tarefas

Uma demo rápida do fluxo de conversa e da execução de tarefas no Blue.

<p align="center">
  <img src="../../docs/assets/demo.gif" alt="Conversation & Task Execution demo" width="800" />
</p>

### Integração de provedores LLM

Uma demo rápida da experiência de integração de provedores LLM no Blue.

<p align="center">
  <img src="../../docs/assets/demo Provider.gif" alt="LLM Providers Integration demo" width="800" />
</p>

### Visão geral rápida - Visão geral, canais e configuração adicional

Uma demo rápida que cobre a visão geral do produto, os canais e a configuração adicional.

<p align="center">
  <img src="../../docs/assets/demo quickv4.gif" alt="Quick Overview demo" width="720" />
</p>

## Por que Blue

<p align="center">
  <img src="../../docs/assets/design_principle.png" alt="Design Principle" />
</p>

### Pure Go, qualquer dispositivo

100% Go, binário estático. Compila cruzada para 5 alvos prontos para uso (![linux](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png) `linux/amd64`, `linux/arm64`, ![macOS](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png) `darwin/amd64`, `darwin/arm64`, ![windows](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png) `windows/amd64`). Sem tempo de execução do Node, sem Python, sem necessidade de contêineres. Coloque-o em um NAS, um ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/RAS.png)Raspberry Pi, um roteador x86 antigo ou um ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)Mac – ele simplesmente funciona. Em seguida, aplique sua própria interface de usuário, lógica e habilidades de agente — uma base de código, cada plataforma.

### Pronto para usar, pronto para trabalhar

Todo mundo quer ferramentas que sejam simples, confiáveis e escaláveis quando você precisar delas. Ferramentas que simplesmente funcionam, para que você possa se concentrar no que realmente está construindo.

Esta não é uma filosofia nova. É o mesmo que construiu <a href="https://www.zimaspace.com/zimaos?utm_source=blue"><img src="https://raw.githubusercontent.com/IceWhaleTech/ZimaOS/main/assets/20241126-153324.png" alt="ZimaOS" height="18" /></a> ZimaOS: simples, confiável e construído para ficar fora do seu caminho. Blue é essa filosofia, estendida à pilha de agentes.

### Projetado para sua vida, construído para permanecer local

Desde pesquisas profundas que fornecem um relatório HTML completo até OCR, PDF, automação de navegador e conversão de documentos, Blue lida com fluxos de trabalho complexos e do mundo real sem enviar seus dados para a nuvem. O despertar por voz, STT/TTS, Talk Mode e o suporte para inferência local tornam as interações cotidianas instantâneas, privadas e sempre disponíveis.

## Início rápido

### Opção 1: Baixe o aplicativo para desktop

Obtenha o aplicativo nativo — sem dependências, sem compilação. Configuração de teste integrada com integração em segundos – comece a conversar instantaneamente por meio de conexão remota, sem necessidade de configuração de bot. Uma verdadeira experiência fora da caixa.

- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)**macOS**: [Baixar DMG](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)
- ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)**Windows**: [Baixar instalador](https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest)

### Opção 2: Instalar script

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
curl -fsSL https://ota.zimaos.com/blue | sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
irm https://ota.zimaos.com/blue/windows | iex
```

### Opção 3: Construir a partir da fonte

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue
git submodule update --init --recursive
```

![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/MAC.png)macOS / ![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/LIN.png)linux
```bash
sh build.sh
```

**![](https://raw.githubusercontent.com/drag-and-publish/operating-system-logos/master/src/16x16/WIN.png)Windows (PowerShell)**
```powershell
.\build.bat
```

> **Observação:** As compilações do Windows exigem:
> - [MinGW-w64](https://www.mingw-w64.org/) (gcc) e [CMake](https://cmake.org/) para dependências C nativas (espeak-ng, Whisper.cpp, opus, kokoro, onnx)
> - [Windows SDK](https://developer.microsoft.com/en-us/windows/downloads/windows-sdk/) para bibliotecas de sistema (winmm, etc.)
>
> Certifique-se de que `gcc`, `cmake` estejam em seu `PATH`.

## Visão geral da arquitetura

<p align="center">
  <img src="../../docs/assets/architecture.png" alt="architecture" />
</p>

Vá além: ele oferece suporte nativo para **mais de 20 plataformas de mensagens instantâneas**, interfaces **acionadas por voz** para diálogo natural e com reconhecimento de contexto, **alteração de modelo com configuração zero** com verificação IDE.

<p align="center">
  <img src="../../docs/assets/providers.png" alt="Supported Providers" />
</p>

## Como construir

<p align="center">
  <img src="../../docs/assets/handcraft.png" alt="handcraft" />
</p>

> ⚠️ [!IMPORTANT]
>
> Se você planeja continuar ajustando ou codificando vibrações em cima do Blue, não trate alguns bate-papos bonitos como evidência de liberação. Qualquer alteração que afete o roteamento, o comportamento de execução, a superfície da ferramenta, o controle de orçamento, a seleção de modelos ou a estrutura de execução deve ser validada com Blue Harness, e não com verificações pontuais ad hoc.
>
> Blue deve seguir uma regra simples aqui: dados primeiro, portas primeiro, corte por último. Na prática, isso significa atualizar o conjunto de dados / especificação de avaliação Harness relevante antes de julgar uma mudança e, em seguida, manter um `candidate_id` estável durante toda a tentativa para que os relatórios de seletor, execução, orçamento e prontidão descrevam o mesmo candidato em vez de quatro execuções não relacionadas.

### Fluxo de trabalho recomendado Harness

1. Execute `blue harness selector verify`
2. Execute `blue harness execution verify`
3. Reutilize a execução de avaliação do seletor para `blue harness budget gate`
4. Finalize com `blue harness cutover-readiness`

Para iteração local, validação noturna ou coleta de evidências de CI, prefira `python3 scripts/cutover_candidate_pipeline.py`. Ele executa o seletor completo -> execução -> orçamento -> sequência de prontidão em um candidato compartilhado, o que torna o resultado mais fácil de comparar, revisar e cortar.

### Guarda-corpos extras

| Área | O que assistir |
|------|----------------|
| Estabilidade da linha de base | Mantenha a linha de base, a versão do conjunto de dados e `candidate_id` estáveis, ou a comparação irá variar e o resultado não será confiável. |
| Resultado de construção real | Recrie o pacote binário ou de front-end afetado antes de executar Harness, caso contrário, você poderá acabar validando um comportamento obsoleto em vez da alteração atual. |
| Cadastro de rota | Se o front-end e o back-end mudarem juntos, confirme se todas as novas rotas de back-end estão realmente registradas antes de julgar o recurso por meio do comportamento da interface do usuário, porque a falta de registro geralmente parece um bug lógico, mas na verdade é um `404`. |
| Liberar julgamento | Uma passagem de ajuste só está pronta quando Harness não mostra nenhuma regressão significativa e a prontidão para transição confirma que o candidato está realmente pronto para a transição. |

Resumindo, sintonizar Blue não é "parece melhor em alguns bate-papos". Trata-se de colocar o candidato em Harness, coletar evidências comparáveis ​​e deixar que os resultados da porta e da prontidão decidam se a mudança é realmente segura de manter.

## Recursos

| Recurso | O que ele oferece |
|--------|-------------------|
| Recuperação da Web de alta disponibilidade e tempo de execução do navegador | Um dos **diferenciais mais nítidos** de Blue. Blue unifica **quatro caminhos de acesso à web** para pesquisa, leitura, extração e rastreamento; mantém **três camadas de fallback** em HTTP, extração de proxy e sessões de navegador; lida com **páginas anti-bot** com detecção de desafios, reutilização de cookies/sessões, furtividade e transferência de navegador; e rotas entre **três mecanismos de navegador**: `lightpanda`, Chromium gerenciado e Chromium de retransmissão/local. |
| Tempo de execução de pesquisa três em um | **Uma entrada de pesquisa pública** pode ser direcionada para `deep_research`, `analyze` e `ui_review`. A mesma pilha de descobertas e evidências produz **pesquisas com citação inicial**, **relatórios limitados** e **revisões estruturadas de UI/UX/acessibilidade**. |
| Harness Estrutura de tempo de execução, avaliação e evolução | Torna a avaliação um **primitivo de tempo de execução** em desenvolvimento, treinamento e produção. Harness cobre **regressão e verificações de fumaça**, pontuação, linhas de base, relatórios e validação de tempo de execução e, em seguida, carrega a mesma evidência para **evolução de habilidades**, avaliação de acompanhamento, promoção ou reversão e `AGENTS.md` ou revisão de proposta de instrução. |
| Tempo de execução multimodal com capacidade nativa primeiro | Mantém **voz, OCR, PDF, tarefas do navegador, conversão de documentos, preenchimento de formulários estruturados, processamento de mídia e geração de mídia** em **caminhos nativos e locais primeiro**, com **roteamento de modelo somente quando for realmente necessário**. |
| Segurança e Governança | Inclui **execução de sandbox**, **defesa contra injeção imediata**, **auditoria de sessão**, permissões, **RBAC**, **WebAuthn**, proteções operacionais e **verificação de segurança de habilidades**. |
| LLM Wiki e Espaço de Conhecimento | Transforma resultados de memória, pesquisa e tempo de execução em uma **superfície de conhecimento semelhante a um wiki** com **páginas de resumo**, índices, **backlinks**, **atualizações** e **fluxos de trabalho de arquivo**. |
| Loja de habilidades e mercado | Fornece **descoberta de habilidades integrada**, curadoria, sincronização e **verificação local** para que a extensibilidade esteja disponível **desde o primeiro dia**. |
| Pool de fornecedores de nível de produção | Fornece um verdadeiro pool de provedores com **verificações de integridade**, **failover automático**, **disjuntores** e **corridas de provedores** para cargas de trabalho de longa duração. |
| Tempo de execução de modelo pequeno local integrado | Fornece um tempo de execução integrado **`Qwen3.5-0.8B` + `llama.cpp`** para **perguntas e respostas curtas locais**, reconhecimento de imagem, roteamento de ferramentas, resumo, **compressão de contexto** e **pré-processamento de documentos**. |
| Confiabilidade de Longa Duração | Trata **OTA atualizações**, **backup e restauração**, **configuração de recarga a quente** e **recuperação pós-falha** como **preocupações operacionais integradas**. |

## Linha do tempo do marco

<p align="center">
  <img src="../../docs/assets/timeline.png" alt="Milestone Timeline" />
</p>

| Data | Versão | Palavras-chave/Recursos |
|------|---------|----------|
| 26 de janeiro de 2026 | `v0.1–v0.9` | Tempo de execução Go, sistema de plugins, automação de navegador |
| 27 a 28 de janeiro de 2026 | `v0.9.0–v0.9.2` | Visualização de tarefas do navegador, Blue Companion, Smart Form Filler |
| 29 a 31 de janeiro de 2026 | `v0.10.0–v0.10.9` | Claude Code CLI, API Proxy, reestruturação da interface do usuário |
| 1 a 3 de fevereiro de 2026 | `v0.10.1–v0.10.22` | Métricas, acesso remoto, cache de contexto |
| 5 a 18 de fevereiro de 2026 | `v0.10.25–v0.10.29` | i18n, CC Cache, pipeline de lançamento |
| 20 a 25 de fevereiro de 2026 | `v0.10.28–v0.10.29` | Carregador de desktop, UX móvel, redesenho de memória |
| 28 de fevereiro a 2 de março de 2026 | `v0.10.30` | Deep Research, reclassificador de habilidades, verificação de segurança |
| 9 a 18 de março de 2026 | `v0.10.31` | Revisão do painel, refatoração VoiceChat, sites aprovados |
| 19 a 22 de março de 2026 | `v0.10.32` | Implementação Harness, auditoria de transcrição, pesquisa na web |
| 23 a 25 de março de 2026 | `v0.10.33` | Grupos Harness, aprovações de navegadores, mercado de habilidades |
| 29 a 30 de março de 2026 | `v0.10.35` | Harness v3, retransmissão de navegador, compactação de contexto |
| 31 de março a 1º de abril de 2026 | `v0.10.36` | Auditoria de transcrição, sobreposições Harness, análise de ferramentas |
| 1º de abril de 2026 | `v0.10.37` | Endurecimento em tempo de execução, corte Skill + Exec, polimento de recuperação |
| 2 a 5 de abril de 2026 | `v0.10.38` | Suporte GitHub, refinamento de mercado, melhorias de confiabilidade |
| 6 a 7 de abril de 2026 | `v0.10.39` | Unificação de pesquisas, superfícies de evolução, redução do uso de memória |

## Comunidade e suporte

- **Problemas**: [Registre bugs e solicitações de recursos aqui](https://github.com/IceWhaleTech/ZimaOS-Blue/issues)
- **Discussões**: [Discord](https://discord.gg/zwWbKA4S2)
- **Siga-nos** em [GitHub](https://github.com/IceWhaleTech)

[![Star History Chart](https://api.star-history.com/svg?repos=IceWhaleTech/ZimaOS-Blue&type=Date)](https://star-history.com/#IceWhaleTech/ZimaOS-Blue&Date)

## Licença

Este projeto está licenciado sob a licença MIT - consulte o arquivo [LICENSE](../../LICENSE) para obter detalhes. Acreditamos no código aberto e na retribuição à comunidade.

## Colaboradores

Obrigado a todos os colaboradores Blue:

<a href="https://community.vaunt.dev/board/IceWhaleTech/repository/ZimaOS-Blue">
  <img src="https://api.vaunt.dev/v1/github/entities/IceWhaleTech/repositories/ZimaOS-Blue/contributors?format=svg&limit=30" width="450" />
</a>

## Referências

1. **OpenClaw** — Primeiro agente de código aberto local. Foi pioneiro na conexão de LLMs a dispositivos locais por meio de adaptadores de canal e chamadas de ferramentas, inspirando diretamente a arquitetura de tempo de execução do agente do Blue. https://github.com/openclaw/openclaw
2. **MiroMind** — Modo de pesquisa profunda com síntese baseada em evidências. Moldou o pipeline de pesquisa profunda integrado do Blue: planejamento, recuperação paralela, desduplicação de evidências e geração de relatórios HTML. https://www.miromind.ai
3. **Karpathy's LLM Wiki** — LLM como compilador de conhecimento. Reestrutura os LLMs para construir espaços de conhecimento persistentes e em evolução, indo além da armadilha de acumulação do RAG.
4. **OpenSpace (HKUDS)** — Mecanismo de habilidades com autoevolução. Uma estrutura baseada em DAG onde os agentes aprendem com as falhas e obtêm habilidades especializadas. https://github.com/HKUDS/OpenSpace
5. **Andrew Ng's Context Hub** — Registro de documentação de API versionada para agentes de codificação. Aborda as alucinações dos agentes e o conhecimento esquecido da sessão. Fornece documentos selecionados e versionados com anotações e ciclos de feedback, transformando a documentação em uma camada de conhecimento de autoaperfeiçoamento. https://github.com/andrewyng/context-hub
6. **Notion** — Simples, humano e intencionalmente silencioso. Inspirado no espírito minimalista de Notion, Blue traz o calor de volta à rede. Onde a serifa refinada encontra um design inteligente, criando um espaço que parece um lar. https://www.notion.com/about
7. **Matrix** — Inspiração visual da icônica estética digital da chuva. A direção estética dos diagramas técnicos do Blue.
8. **IceWhale** — Amor, Morte e Robôs S2E2 "Gelo". Um coletivo que se reúne em todo o mundo para romper os muros dos gigantes da internet e resistir à concentração de dados. A baleia de gelo simboliza uma comunidade que constrói ferramentas soberanas no limite.
9. **ZimaOS Blue** — Amor, Morte e Robôs S1E14 "Zima Blue". Uma metáfora: inteligência que começa no serviço e evolui para explorar o mundo. Blue é um agente de sabedoria, enraizado na simplicidade e buscando profundidade.
10. **ZimaOS** — Princípios de design simplificados, focados e abertos. Tanto ZimaOS quanto Blue compartilham a crença de que a tecnologia deve servir ao usuário – implantar em 30 segundos, executar em qualquer lugar, permanecer neutro em relação ao fornecedor. https://www.zimaspace.com/zimaos
