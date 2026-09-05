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
- [Segurança](#segurança)
- [Arquitetura](#arquitetura)
- [Onde os dados ficam](#onde-os-dados-ficam)
- [Importação / Exportação](#importação--exportação)
- [Atalhos de teclado](#atalhos-de-teclado)
- [Build](#build)
- [Estrutura do projeto](#estrutura-do-projeto)
- [Testes](#testes)
- [Roadmap](#roadmap)
- [Licença](#licença)

---

## Recursos

### Gerenciamento de itens
- **4 tipos de item**: Login, Nota segura, Cartão de crédito e Identidade (campos dinâmicos por tipo).
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
- **Sincronização** — pasta local/NAS e Google Drive (OAuth), com cópias de conflito automáticas.
- **Extensão de navegador** — Chrome/Firefox via host nativo (preenchimento, TOTP, pareamento).
- **Auto-lock** configurável (1/5/15/30/60 min ou nunca) e bloqueio ao minimizar a janela.
- **Temas** Dark / Light / Sistema com persistência.
- **Importação** de Chrome, Firefox, Edge, LastPass, 1Password, Bitwarden e KeePass (.kdbx).
- **Exportação** em CSV, JSON e transferência criptografada `.passapp` entre instâncias do CipherSync.
- **Exclusão de conta** com confirmação por digitação (`DELETAR TUDO`).

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
│         │  Engine  │  │ (SQLite)│  │ (fut.)│  │
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

- **Windows**: `%APPDATA%\CipherSync\`
- **Linux**: `~/.config/CipherSync/`

```
CipherSync/
├── pessoal.passapp        # cofre criptografado (SQLite)
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

O export gera **CSV**, **JSON** (com aviso de segurança) ou **transferência criptografada** com senha.

---

## Atalhos de teclado

| Atalho | Ação |
|--------|------|
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
| `Esc` | Fechar modal / limpar seleção |

---

## Build

Pré-requisitos: **Go 1.25+**, **Node 20+**, **Wails CLI** (e [WebView2](https://developer.microsoft.com/en-us/microsoft-edge/webview2/) no Windows).

```bash
# instala o CLI do Wails (uma vez)
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# desenvolvimento com hot reload
wails dev

# build de produção (Windows: gera .exe + instalador NSIS se disponível)
wails build

# build limpo
wails build -clean
```

O executável é gerado em `build/bin/`.

### Linux

- Dependências nativas: `libgtk-3-dev`, `libwebkit2gtk-4.0-dev` (ou `-4.1`) e `build-essential`.
- Clipboard/favicons usam ferramentas padrão do desktop (xclip/xsel ou wl-clipboard no Wayland).

### Ícone

Para regenerar os ícones a partir de `ciphersync-logo.png`:

```powershell
.\make_icon.ps1
```

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
├── generator.go          # gerador de senhas/frases
├── wordlist.go           # wordlist para frases
├── favicon.go            # fetch de favicons com cache
├── main.go               # entrada do app
├── frontend/
│   └── src/
│       ├── components/   # UI (ItemDetail, Watchtower, TOTP, modais...)
│       ├── lib/          # api, types, theme, autolock
│       └── state.ts      # store Zustand
├── testdata/             # arquivos de teste para import
├── make_icon.ps1         # gera ícones a partir da logo
└── wails.json            # configuração do Wails
```

---

## Google Drive (sincronização)

1. Crie um projeto no [Google Cloud Console](https://console.cloud.google.com/) e um **OAuth client ID** do tipo **Desktop app**.
2. No app: abra um cofre → **Sync** (sidebar) → escolha **Google Drive**.
3. Cole o **Client ID** (e o Client Secret, se houver) e clique **Conectar Google** — o navegador abre para autorizar.
4. Clique **Criar/usar pasta** para criar a pasta `CipherSync` no Drive e ativar o sync.
5. Os tokens ficam no gerenciador de credenciais do SO (nunca em arquivo).

> A primeira conexão pode exibir o aviso "app não verificado" do Google (tela de consentimento em modo de teste) — é esperado até a verificação do app.

---

## Testes

```bash
go test ./...
```

Os testes cobrem: ciclo de vida do cofre, troca de senha mestra, migração de schema, batch operations, lixeira (trash/restore/purge/retenção), anexos, backup, múltiplos cofres, import/export (CSV, Bitwarden, KeePass, transferência), TOTP/QR e análise do Watchtower.

Arquivos de teste de import em `testdata/`:
- `1password_export.csv` — 50 cadastros no formato do 1Password
- `chrome_passwords.csv` — 26 cadastros no formato do Chrome/Edge
- `bitwarden_export.json` — 20 itens multi-tipo (6 logins, 5 notas, 5 cartões, 4 identidades)

---

## Roadmap

- **Fase 5** — Acesso: system tray, quick access global (Ctrl+Shift+Espaço), passkeys (inventário), i18n EN + PT-BR
- **Fase 6** — Sincronização: engine + pasta local/NAS + Google Drive, com resolução de conflitos
- **Fase 7** — Extensão de navegador (Chrome/Firefox) via host nativo
- **Futuro** — Campos customizados, travel mode, emergency kit, compartilhamento, CI/CD

---

## Licença

Distribuído sob a licença **MIT** — veja [LICENSE](LICENSE).

---

<div align="center">

Feito com 🧠, 🔐 e Go.

</div>
