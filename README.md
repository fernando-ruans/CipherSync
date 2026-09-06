<div align="center">

<img src="ciphersync-logo.png" alt="CipherSync" width="160"/>

# CipherSync

**Gerenciador de senhas open-source para Windows e Linux, inspirado no 1Password.**

Criptografia local de ponta a ponta, sem servidores, sem telemetria — seus dados pertencem a você.

`Go` `Wails v2` `React` `TypeScript` `Tailwind CSS` `SQLite` `Argon2id` `AES-256-GCM`

</div>

---

## Sumário

- [Recursos](#recursos)
- [Instalação e build](#instalação-e-build)
- [Extensão de navegador — passo a passo](#extensão-de-navegador--passo-a-passo)
- [Segurança](#segurança)
- [Arquitetura](#arquitetura)
- [Onde os dados ficam](#onde-os-dados-ficam)
- [Importação / Exportação](#importação--exportação)
- [Sincronização](#sincronização)
- [Atalhos de teclado](#atalhos-de-teclado)
- [Estrutura do projeto](#estrutura-do-projeto)
- [Testes](#testes)
- [Correções recentes](#correções-recentes)
- [Roadmap](#roadmap)
- [Licença](#licença)

---

## Recursos

### Gerenciamento de itens
- **5 tipos de item**: Login, Nota segura, Cartão de crédito, Identidade e **Passkey** (inventário FIDO2).
- CRUD completo, busca em tempo real (título, usuário, URL, notas, tags e campos), favoritos, categorias e **tags** com autocomplete.
- **Histórico de versões**: snapshot a cada alteração (até 50 por item), diff visual e restauração.
- **Lixeira**: exclusões vão para a lixeira (restaurar ou purgar), com retenção configurável e limpeza automática.
- **Anexos**: arquivos criptografados por item (máx. 10 MB), com download e remoção.
- **Multi-seleção e operações em lote**: lixeira, mover para categoria, adicionar tag, favoritar e exportar os itens selecionados.
- **Favicons** carregados automaticamente com cache local (TTL de 30 dias).

### Segurança avançada
- **TOTP / 2FA integrado** — autenticador de 6 dígitos com QR code por **câmera**, **upload de imagem** ou **chave manual**, com código inline na lista e círculo de contagem regressiva.
- **Watchtower** — painel de saúde das senhas com análise **zxcvbn** (fracas, duplicadas com máscara, antigas, sem 2FA e vazadas), score geral de 0–100%.
- **Detecção de vazamento (HIBP)** — verifica se suas senhas já vazaram usando **k-anonymity** (apenas os 5 primeiros caracteres do SHA-1 saem da sua máquina).
- **Gerador de senhas** — aleatórias (comprimento/tipos), **presets de PIN** (4/6 dígitos) e frases por palavras.
- **Backups automáticos** — snapshot diário ao desbloquear (mantidos os 10 mais recentes) + botão de backup manual.

### Experiência
- **Múltiplos cofres** (pessoal, trabalho, família), cada um com sua própria senha mestra, com seletor na tela de desbloqueio.
- **System tray** — ícone na bandeja com Mostrar, Bloquear, Gerar senha e Sair; fechar minimiza para a bandeja (configurável).
- **Quick Access** — popup global de busca com `Ctrl+Shift+Space`, mesmo com outro app focado.
- **Passkeys** — inventário de credenciais FIDO2 (referência, sem autenticação WebAuthn).
- **Idiomas** — Português (BR) e English, com seletor em Configurações.
- **Sincronização** — pasta local/NAS, com cópias de conflito automáticas.
- **Extensão de navegador** — Chrome/Firefox via host nativo (preenchimento, TOTP, pareamento).
- **Auto-lock** configurável (1/5/15/30/60 min ou nunca) — o tempo conta mesmo com a janela oculta.
- **Temas** Dark / Light / Sistema com persistência.
- **Importação** de Chrome, Firefox, Edge, LastPass, 1Password, Bitwarden e KeePass (.kdbx).
- **Exportação** em CSV, JSON e transferência criptografada `.passapp` entre instâncias do CipherSync.
- **Exclusão de conta** com confirmação por digitação (`DELETAR TUDO`).

---

## Instalação e build

### Para usuários (sem compilar)

Baixe o executável mais recente em `build/bin/CipherSync.exe` (Windows) ou compile seguindo os passos abaixo. Requisitos de sistema: Windows 10+ com [WebView2](https://developer.microsoft.com/en-us/microsoft-edge/webview2/) (já incluído no Windows 11) ou Linux com GTK3/WebKit.

### Para desenvolvedores

Pré-requisitos: **Go 1.25+**, **Node 20+**, **Wails CLI** (e [WebView2](https://developer.microsoft.com/en-us/microsoft-edge/webview2/) no Windows).

```bash
# clone e entre no projeto
git clone <url-do-repositorio>
cd PassApp

# instala o CLI do Wails (uma vez)
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# instala as dependências do frontend
cd frontend && npm install && cd ..

# desenvolvimento com hot reload
wails dev

# build de produção (Windows: gera .exe + instalador NSIS se disponível)
wails build

# build limpo
wails build -clean
```

O executável é gerado em `build/bin/`.

#### Linux

- Dependências nativas: `libgtk-3-dev`, `libwebkit2gtk-4.0-dev` (ou `-4.1`) e `build-essential`.
- Clipboard/favicons usam ferramentas padrão do desktop (xclip/xsel ou wl-clipboard no Wayland).

#### Ícone

Para regenerar os ícones a partir de `ciphersync-logo.png`:

```powershell
.\make_icon.ps1
```

---

## Extensão de navegador — passo a passo

A extensão (Chrome/Edge/Firefox, pasta `extension/`) conversa com o app via **host nativo**. Tudo roda em loopback com token efêmero — nada é exposto na rede.

### 1. Carregue a extensão no navegador

1. Abra `chrome://extensions` (ou `about:debugging#/runtime/this-firefox` no Firefox).
2. Ative **Modo do desenvolvedor**.
3. Clique em **Carregar sem compactação** e selecione a pasta `extension/` do projeto.
4. Copie o **ID da extensão** exibido no cartão da extensão.

### 2. Instale o host nativo no CipherSync

1. Abra o CipherSync e desbloqueie o cofre.
2. Vá em **Configurações → Extensão do navegador**.
3. Cole o **ID da extensão** copiado no passo 1.
4. Clique em **Instalar host nativo** (registra o manifest do `CipherSync.exe --native-host` para o seu usuário).

### 3. Pareie a extensão com o app

1. Ainda em **Configurações → Extensão**, clique em **Gerar código de pareamento** (o código expira em 10 minutos).
2. No navegador, abra **Opções** da extensão (botão direito no ícone → Opções).
3. Cole o código e clique em **Parear**. A extensão guarda um ID de associação local.

### 4. Use

- Em uma página de login, clique no **ícone da extensão**: ela busca no cofre os logins do mesmo domínio (eTLD+1) e permite copiar usuário/senha/TOTP ou preencher o formulário.
- Ao criar uma senha em um site novo, a extensão oferece **salvar no CipherSync** (cria ou atualiza o login).
- O cofre **precisa estar desbloqueado**; bloqueado, a extensão avisa e não acessa nada.
- **Desinstalar o host**: botão **Remover host nativo** na mesma tela de configurações.

> A comunicação usa um token aleatório gravado em `%APPDATA%\LockSync\.localapi.json` (só loopback, permissão 0600) e códigos de pareamento de uso único — regeneráveis a qualquer momento.

---

## Segurança

O modelo de segurança segue as práticas de gerenciadores consolidados (modelo 1Password):

```
Senha mestra (nunca armazenada)
        │
        ▼  Argon2id (64 MiB, 4 iterações, 4 threads)
Chave mestra derivada
        │
        ▼  AES-256-GCM
Chave do cofre (vault key) ──► criptografa cada item individualmente
```

- **Senha mestra** nunca é armazenada; a chave do cofre é derivada via **Argon2id** e fica apenas em memória.
- Cada cofre é um arquivo `<nome>.passapp` (SQLite) com cada item criptografado individualmente com **AES-256-GCM** (nonce único por item).
- A chave do cofre é criptografada pela chave derivada da senha mestra — **trocar a senha mestra não re-criptografa os itens**.
- Chaves e buffers são **zerados em memória** ao bloquear o cofre.
- Clipboard limpo automaticamente após **60 segundos**.
- **HIBP** usa k-anonymity: apenas os 5 primeiros caracteres do hash SHA-1 da senha são enviados à API pública — nenhuma senha ou hash completo sai da máquina.
- Banco de dados em SQLite **puro Go** (modernc.org/sqlite), sem dependência de CGO — facilita o cross-compile para Linux.

---

## Arquitetura

```
┌───────────────────────────────────────────────┐
│                CipherSync (Wails)              │
│  ┌──────────────────┐   ┌───────────────────┐  │
│  │   React + TS UI  │◄──┤    Go Backend     │  │
│  │  (Tailwind,     │   │   (bindings)      │  │
│  │   Zustand)      │   └────────┬──────────┘  │
│  └──────────────────┘            │            │
│               │            ┌─────┼────────┐   │
│         ┌─────▼────┐  ┌────▼────┐  ┌────▼─┐  │
│         │  Crypto  │  │  Vault  │  │ Sync │  │
│         │  Engine  │  │ (SQLite)│  │      │  │
│         └──────────┘  └─────────┘  └──────┘  │
└───────────────────────────────────────────────┘
```

| Camada | Tecnologia |
|--------|-----------|
| Backend | Go 1.25+ com Wails v2 |
| Frontend | React 19 + TypeScript |
| Estilo | Tailwind CSS v4 |
| Estado | Zustand |
| Banco | SQLite via modernc.org/sqlite (puro Go, sem CGO) |
| Crypto | golang.org/x/crypto (Argon2id), AES-256-GCM |
| TOTP/QR | pquerna/otp + skip2/go-qrcode |
| Scanner QR | jsqr (webcam/upload no frontend) |
| Força de senha | zxcvbn (Go + JS) |
| Clipboard | atotto/clipboard com auto-clear condicional |

---

## Onde os dados ficam

Os cofres ficam no diretório de configuração do usuário:

- **Windows**: `%APPDATA%\LockSync\`
- **Linux**: `~/.config/LockSync/`

> ℹ️ O nome de pasta `LockSync` é mantido por compatibilidade com instalações anteriores ao rebranding — nada muda para quem já usava o app.

```
LockSync/
├── pessoal.passapp        # cofre criptografado (SQLite)
├── pessoal.sync.json      # estado de sincronização (se configurada)
├── backups/               # snapshots diários (10 mais recentes)
└── trabalho.passapp       # múltiplos cofres suportados
```

> ⚠️ Os arquivos `.passapp` são criptografados, mas **faça backups** do diretório acima. Sem a senha mestra, os dados são irrecuperáveis.

---

## Importação / Exportação

O import é feito pelo menu **Importar** na sidebar:

| Formato | Detecção | Tipos de item |
|---------|----------|---------------|
| CSV Chrome / Edge / Brave | Automática por cabeçalho | Login |
| CSV Firefox | Automática (título derivado do domínio) | Login |
| CSV LastPass | Automática | Login |
| CSV 1Password | Automática | Login |
| CSV genérico (`;` incluído) | Mapeamento manual de colunas | Login |
| Bitwarden JSON | Nativa | Login, Nota, Cartão, Identidade |
| KeePass (.kdbx v3/v4) | Com senha do banco | Login |
| Transferência CipherSync (`.passapp`) | Criptografada | Todos |

O export gera **CSV**, **JSON** (com aviso de segurança; o material privado de passkeys nunca é incluído) ou **transferência criptografada** com senha.

---

## Sincronização

O sync é **arquivo-inteiro com LWW (last-write-wins)** e cópias de conflito:

1. Configure em **Configurações → Sincronização** uma pasta local/NAS (ex.: pasta do Dropbox/Syncthing/pendrive).
2. A cada alteração relevante (e a cada 60 s em segundo plano) o app faz um snapshot consistente do cofre (`VACUUM INTO`) e envia para a pasta remota.
3. Se ambos os lados mudaram, o mais novo vence e o **perdedor é preservado** como `meuCofre (conflict data, máquina).passapp` — nada é perdido silenciosamente.
4. Estados corrompidos de sync são reavaliados por **comparação de conteúdo** (hash), evitando conflitos falsos.
5. Desconectar o sync não apaga nada — só para a sincronização.

> Snapshots do sync ficam em `<cofre>.sync.json` ao lado do cofre (não dentro do banco).

---

## Atalhos de teclado

| Atalho | Ação |
|--------|------|
| `Ctrl+Shift+Space` | Quick Access global (busca + copiar senha de qualquer app) |
| `Ctrl+N` | Novo item |
| `Ctrl+F` | Focar na busca |
| `Ctrl+S` | Salvar item em edição |
| `Ctrl+B` | Copiar senha do item selecionado |
| `Ctrl+Shift+C` | Copiar usuário do item selecionado |
| `Ctrl+G` | Abrir gerador de senhas |
| `Ctrl+A` | Selecionar todos os itens visíveis |
| `Ctrl+D` | Mover itens selecionados para a lixeira |
| `Ctrl+Shift` + clique | Seleção em intervalo |
| `Ctrl+clique` | Selecionar item individual |
| `Ctrl+L` | Bloquear o cofre |
| `Esc` | Fechar modal / limpar seleção |

---

## Estrutura do projeto

```
.
├── app.go                # bindings Wails (API do frontend)
├── crypto.go             # Argon2id + AES-256-GCM
├── vault.go              # cofre SQLite, CRUD, versões, batch ops
├── vaults.go             # múltiplos cofres, slugify, listagem
├── totp.go               # TOTP/2FA
├── watchtower.go         # análise de saúde + HIBP
├── import_export.go      # import/export + transferência criptografada
├── passkey.go            # validação de credenciais FIDO2 (inventário)
├── generator.go          # gerador de senhas/frases
├── wordlist.go           # wordlist para frases
├── favicon.go            # fetch de favicons com cache
├── sync.go / sync_local.go  # engine de sincronização (LWW + conflitos)
├── tray.go               # system tray
├── hotkey_windows.go     # atalho global Ctrl+Shift+Space
├── localapi.go           # API loopback (token) usada pela extensão
├── nativehost.go         # host nativo de messaging (stdin/stdout)
├── nativeinstall*.go     # instalação dos manifests por navegador/OS
├── main.go               # entrada do app
├── frontend/
│   └── src/
│       ├── components/   # UI (ItemDetail, Watchtower, TOTP, modais...)
│       ├── lib/          # api, types, i18n, theme, autolock
│       └── state.ts      # store Zustand
├── extension/            # extensão Chrome/Firefox (MV3)
├── testdata/             # arquivos de teste para import
├── make_icon.ps1         # gera ícones a partir da logo
└── wails.json            # configuração do Wails
```

---

## Testes

```bash
go test ./...
```

Os testes cobrem: ciclo de vida do cofre, troca de senha mestra, reabertura por chave (reload pós-sync), migração de schema, batch operations, lixeira (trash/restore/purge em lote/retenção), anexos, backup + pruning, múltiplos cofres, import/export (CSV, Bitwarden, transferência), validação de passkeys, export JSON sem material privado, transfer com payload malicioso, TOTP (bindings + URI + QR), API local da extensão (scoping por domínio, upsert, auth), pareamento com expiração/GC, sync fim-a-fim (upload/download/conflito/rollback) e análise do Watchtower.

Arquivos de teste de import em `testdata/`:
- `1password_export.csv` — 50 cadastros no formato do 1Password
- `chrome_passwords.csv` — 26 cadastros no formato do Chrome/Edge
- `bitwarden_export.json` — 20 itens multi-tipo (6 logins, 5 notas, 5 cartões, 4 identidades)

---

## Correções recentes

### 2026-09-06 — Navegação e troca de cofres

- **Botão de retorno na criação de cofre**: a tela "Criar seu cofre" agora tem sempre o link "‹ Todos os cofres", eliminando estados em que o usuário ficava preso sem conseguir voltar ou logar.
- **Exclusão de cofre corrigida**: deletar um cofre não redireciona mais para a tela de criação — o app permanece na lista de cofres (mesmo vazia), com o botão "Novo cofre" sempre disponível.
- **Troca de cofre atualiza a lista**: ao bloquear ("Trocar de cofre"), a lista de cofres é recarregada do backend — cofres criados durante a sessão aparecem imediatamente, sem precisar fechar e reabrir o app.
- **Inicialização mais robusta**: se a listagem de cofres falhar ao abrir o app, cai na tela de desbloqueio (que oferece o caminho "Novo cofre") em vez da tela de criação.

---

## Roadmap

**Concluído:**
- ✅ Fases 1–4 — cofre criptografado, tipos de item, versionamento, lixeira, anexos, backups, import/export, TOTP, Watchtower/HIBP, multi-cofres, rebranding
- ✅ Fase 5 — system tray, Quick Access global, passkeys (inventário), i18n (PT-BR/EN)
- ✅ Fase 6 — sincronização por pasta local/NAS com resolução de conflitos
- ✅ Fase 7 — extensão de navegador via host nativo (Chrome/Firefox)

**Futuro:**
- Campos customizados por item
- Travel mode e emergency kit
- Autenticação WebAuthn de verdade (passkeys Scope B)
- Compartilhamento de cofres
- CI/CD com builds assinados

---

## Licença

Distribuído sob a licença **MIT** — veja [LICENSE](LICENSE).

---

<div align="center">

Feito com 🧠, 🔐 e Go.

</div>
