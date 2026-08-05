# CipherSync

Gerenciador de senhas open-source para Windows e Linux, inspirado no 1Password.

## Stack

| Camada | Tecnologia |
|--------|-----------|
| Backend | Go + Wails v2 |
| Frontend | React 19 + TypeScript + Tailwind CSS v4 |
| Estado | Zustand |
| Banco | SQLite (modernc.org/sqlite — puro Go, sem CGO) |
| Crypto | Argon2id + AES-256-GCM |
| Clipboard | Auto-clear após 60s |

## Segurança

- **Senha mestra** nunca é armazenada; a chave do cofre é derivada via Argon2id e fica apenas em memória.
- **Cofres locais** — cada cofre é um arquivo `<nome>.passapp` criptografado, com cada item criptografado individualmente com AES-256-GCM.
- Chave do cofre é criptografada pela chave derivada da senha mestra (troca de senha não re-criptografa os itens).
- Chaves e buffers são zerados ao bloquear o cofre.
- Clipboard limpo automaticamente após 60 segundos.
- **Detecção de vazamento (HIBP)** usa k-anonymity: apenas os 5 primeiros caracteres do SHA-1 da senha saem da máquina.

## Funcionalidades

### Fase 1 — MVP

- [x] Criação do cofre com senha mestra (indicador de força)
- [x] Desbloqueio/bloqueio do cofre
- [x] CRUD de itens (título, usuário, senha, URL, notas, categoria, favorito)
- [x] Busca em memória (título, usuário, URL, notas)
- [x] Gerador de senhas aleatórias e por frases (palavras)
- [x] Copiar para a área de transferência com auto-clear
- [x] Tema escuro, atalhos `Ctrl+N` (novo), `Ctrl+L` (bloquear)
- [x] Alterar senha mestra

### Fase 2 — Funcionalidades Essenciais

- [x] **Tipos de item** — Login, Nota segura, Cartão de crédito, Identidade (campos dinâmicos por tipo)
- [x] **Sistema de tags** — input com autocomplete, chips, filtro na sidebar
- [x] **Favicons** — fetch automático com cache local (evento em tempo real)
- [x] **Histórico de versões** — snapshot a cada alteração (até 50/item), diff visual e restore
- [x] **Import** — CSV genérico com mapeamento de colunas, CSV auto-detectado (Chrome, Edge, Firefox, LastPass, 1Password), Bitwarden JSON (todos os tipos de item), transferência CipherSync criptografada
- [x] **Export** — CSV, JSON (com aviso de segurança) e transferência criptografada `.passapp`
- [x] **Temas** — Dark / Light / Sistema, com persistência
- [x] **Auto-lock** — 1/5/15/30/60 min ou nunca; bloqueia ao minimizar
- [x] **Atalhos** — `Ctrl+F` busca, `Ctrl+S` salvar, `Ctrl+B` copiar senha, `Ctrl+Shift+C` copiar usuário, `Esc` fecha modais

### Fase 3 — Gerenciamento

- [x] **Multi-seleção** — checkboxes na lista, `Ctrl+A` (todos), `Ctrl+Click` (individual), `Shift+Click` (intervalo)
- [x] **Operações em lote** — excluir (`Ctrl+D`), mover para categoria, adicionar tag, favoritar/desfavoritar e exportar (CSV/JSON) os itens selecionados
- [x] **Múltiplos cofres** — crie quantos cofres quiser (pessoal, trabalho, família), cada um com sua senha mestra; seletor na tela de desbloqueio e "Trocar de cofre" na sidebar
- [x] **Exclusão de conta** — apaga todos os cofres e dados, resetando o app; exige digitar `DELETAR TUDO` para confirmar

### Fase 4 — Segurança Avançada

- [x] **TOTP/2FA integrado** — autenticador de 6 dígitos com QR code por câmera, upload de imagem ou chave manual
- [x] **Watchtower** — painel de saúde das senhas (fracas, duplicadas, antigas, sem 2FA)
- [x] **Detecção de vazamento (HIBP)** — verifica se senhas já vazaram via k-anonymity
- [x] **Windows Hello** — desbloqueio biométrico via DPAPI (Windows)
- [x] **Logotipo oficial** — ícone do app gerado a partir do logo da marca

## Roadmap

- **Fase 5** — Sincronização pluggable (arquivo, Dropbox, Google Drive, WebDAV) com resolução de conflitos
- **Fase 6** — Campos customizados, anexos, travel mode, emergency kit, compartilhamento
- **Fase 7** — System tray, quick access, extensão de navegador, backups, CI/CD

## Desenvolvimento

Pré-requisitos: Go 1.25+, Node 20+, Wails CLI.

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# desenvolvimento (hot reload)
wails dev

# build de produção
wails build
```

O executável é gerado em `build/bin/`.

> No Linux, o gerador de senhas e o clipboard usam ferramentas padrão do desktop
> (xclip/xsel ou wl-clipboard no Wayland).

## Testes

```bash
go test ./...
```

- `testdata/1password_export.csv` — exportação de exemplo no formato padrão do 1Password (50 cadastros) para testar o import.
- `testdata/chrome_passwords.csv` — exportação de exemplo no formato do Chrome/Edge (26 cadastros) para testar o import de navegador.
- `testdata/bitwarden_export.json` — exportação Bitwarden multi-tipo (6 logins, 5 notas, 5 cartões, 4 identidades) com categorias.

## Licença

MIT — veja [LICENSE](LICENSE).
