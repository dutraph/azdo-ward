# azdo-ward 🔭

> A clear web viewer for Azure DevOps audit-log changes · Visualizador web para mudanças no audit log do Azure DevOps · Visor web para los cambios del registro de auditoría de Azure DevOps

<p align="center">
  <strong><a href="#português">Português</a></strong>
  ·
  <strong><a href="#english">English</a></strong>
  ·
  <strong><a href="#español">Español</a></strong>
</p>

`azdo-ward` is a single self-contained Go binary that serves a local web UI. It reads the Azure DevOps **Audit Log** (`vso.auditlog` scope) and renders the changes as a filterable timeline, with before/after diffs, grouping, search and CSV/JSON export. Your PAT stays on the machine — it lives in the local config and is proxied server-side, never exposed to the browser.

![layout](https://img.shields.io/badge/stack-Go%20%2B%20embedded%20SPA-4493f8) ![license](https://img.shields.io/badge/license-Apache--2.0-3fb950)

---

## Português

Visualizador web para as mudanças logadas no audit do Azure DevOps. Um binário Go único sobe um servidor local que consulta a Audit API ao vivo e mostra os eventos como uma timeline com filtros, diff antes/depois, agrupamento, busca e export.

### Instalação

#### Opção A — `go install`

```bash
go install github.com/dutraph/azdo-ward/cmd/azdo-ward@latest
```

#### Opção B — script de instalação

```bash
curl -fsSL https://raw.githubusercontent.com/dutraph/azdo-ward/main/install.sh | bash
```

#### Opção C — build local

```bash
git clone https://github.com/dutraph/azdo-ward.git
cd azdo-ward
make install         # auto-bump do patch + instala em /usr/local/bin
make run             # build e sobe o servidor
```

### Pré-requisitos

- Go 1.22+
- Um **Personal Access Token** do Azure DevOps com o escopo **Audit Log (Read)** (`vso.auditlog`). Crie em `https://dev.azure.com/<org>/_usersSettings/tokens`.

### Uso

```bash
azdo-ward connect <org> <pat>     # guarda a org + PAT (multi-conta)
azdo-ward                         # sobe a UI em http://127.0.0.1:7878
azdo-ward serve --addr :9000      # porta custom
azdo-ward orgs                    # lista as orgs configuradas (★ = ativa)
azdo-ward switch <org>            # troca a org ativa
```

Abra o navegador, escolha a janela de tempo (24h/7d/30d/90d ou datas), clique **Load**, e use os filtros (categoria, área, ator, busca) e o **Group by**. Eventos com pares de valor antigo/novo no `data` ganham um diff visual; qualquer visão filtrada pode ser exportada em CSV ou JSON.

> O PAT é gravado em `~/Library/Application Support/azdo-ward/config.yaml` (macOS) ou `$XDG_CONFIG_HOME/azdo-ward/config.yaml` (Linux), com permissão `0600`, e nunca é enviado ao navegador.

<p align="right"><a href="#azdo-ward">▲ back to top</a></p>

---

## English

A clear web viewer for the changes logged in the Azure DevOps audit log. A single Go binary serves a local server that queries the live Audit API and renders events as a filterable timeline with before/after diffs, grouping, search and export.

### Installation

#### Option A — `go install`

```bash
go install github.com/dutraph/azdo-ward/cmd/azdo-ward@latest
```

#### Option B — install script

```bash
curl -fsSL https://raw.githubusercontent.com/dutraph/azdo-ward/main/install.sh | bash
```

#### Option C — local build

```bash
git clone https://github.com/dutraph/azdo-ward.git
cd azdo-ward
make install
make run
```

### Prerequisites

- Go 1.22+
- An Azure DevOps **Personal Access Token** with the **Audit Log (Read)** scope (`vso.auditlog`). Create one at `https://dev.azure.com/<org>/_usersSettings/tokens`.

### Usage

```bash
azdo-ward connect <org> <pat>     # store org + PAT (multi-account)
azdo-ward                         # serve the UI at http://127.0.0.1:7878
azdo-ward serve --addr :9000      # custom port
azdo-ward orgs                    # list configured orgs (★ = active)
azdo-ward switch <org>            # change the active org
```

Open the browser, pick a time window (24h/7d/30d/90d or explicit dates), click **Load**, then use the filters (category, area, actor, search) and **Group by**. Events whose `data` carries old/new value pairs render a visual diff; any filtered view can be exported as CSV or JSON.

> The PAT is written to `~/Library/Application Support/azdo-ward/config.yaml` (macOS) or `$XDG_CONFIG_HOME/azdo-ward/config.yaml` (Linux) with `0600` perms, and is never sent to the browser.

<p align="right"><a href="#azdo-ward">▲ back to top</a></p>

---

## Español

Visor web para los cambios registrados en el audit log de Azure DevOps. Un único binario de Go levanta un servidor local que consulta la Audit API en vivo y muestra los eventos como una línea de tiempo con filtros, diff antes/después, agrupación, búsqueda y exportación.

### Instalación

#### Opción A — `go install`

```bash
go install github.com/dutraph/azdo-ward/cmd/azdo-ward@latest
```

#### Opción B — script de instalación

```bash
curl -fsSL https://raw.githubusercontent.com/dutraph/azdo-ward/main/install.sh | bash
```

#### Opción C — compilación local

```bash
git clone https://github.com/dutraph/azdo-ward.git
cd azdo-ward
make install
make run
```

### Requisitos

- Go 1.22+
- Un **Personal Access Token** de Azure DevOps con el alcance **Audit Log (Read)** (`vso.auditlog`). Créalo en `https://dev.azure.com/<org>/_usersSettings/tokens`.

### Uso

```bash
azdo-ward connect <org> <pat>     # guarda la org + PAT (multicuenta)
azdo-ward                         # sirve la UI en http://127.0.0.1:7878
azdo-ward serve --addr :9000      # puerto personalizado
azdo-ward orgs                    # lista las orgs configuradas (★ = activa)
azdo-ward switch <org>            # cambia la org activa
```

Abre el navegador, elige una ventana de tiempo (24h/7d/30d/90d o fechas), pulsa **Load** y usa los filtros (categoría, área, actor, búsqueda) y **Group by**. Los eventos cuyo `data` contiene pares de valor antiguo/nuevo muestran un diff visual; cualquier vista filtrada se puede exportar en CSV o JSON.

> El PAT se guarda en `~/Library/Application Support/azdo-ward/config.yaml` (macOS) o `$XDG_CONFIG_HOME/azdo-ward/config.yaml` (Linux) con permisos `0600`, y nunca se envía al navegador.

<p align="right"><a href="#azdo-ward">▲ back to top</a></p>

---

## Authentication · Autenticação · Autenticación

azdo-ward supports two auth modes per organization:

**PAT** — a Personal Access Token with the **Audit Log (Read)** scope (`vso.auditlog`), stored locally and never sent to the browser.

**Entra** — for organizations that **block PAT auth by policy** (common in enterprises on Microsoft Entra ID). No secret is stored; azdo-ward mints an Entra access token on demand via the Azure CLI. Run `az login` once, then:

```bash
azdo-ward connect <org> --entra      # CLI
# or pick "Azure Entra ID (az login)" in the web connect screen
```

> Detecting a PAT-blocked org: a query that redirects to `_signin` (HTTP 302) instead of returning JSON means PAT auth was rejected — switch that org to Entra.

## Architecture

```
azdo-ward/
├── cmd/azdo-ward/main.go        # CLI entry: serve / connect / orgs / switch
└── internal/
    ├── api/                     # hand-rolled Audit API client (PAT basic auth)
    │   ├── client.go            # Query + QueryAll (continuation-token paging)
    │   └── types.go             # DecoratedAuditLogEntry / QueryResult
    ├── auth/entra.go            # Entra ID token provider (az CLI, cached)
    ├── config/config.go         # multi-account YAML in $XDG_CONFIG_HOME
    ├── server/server.go         # HTTP routes + /api/audit proxy
    ├── version/version.go       # ldflags-stamped version
    └── web/                     # embedded single-page frontend
        ├── embed.go             # go:embed static
        └── static/{index.html, app.js, styles.css}
```

The frontend is dependency-free vanilla JS; the backend depends only on `gopkg.in/yaml.v3`. Releases are built by `.github/workflows/release.yml` (darwin/linux × amd64/arm64) on `v*` tags.

## Audit API reference

Built against the Azure DevOps Audit **Query** endpoint, `api-version=7.1-preview.1`:
`GET https://auditservice.dev.azure.com/{org}/_apis/audit/auditlog`. See the [Microsoft Learn reference](https://learn.microsoft.com/en-us/rest/api/azure/devops/audit/audit-log/query?view=azure-devops-rest-7.1).

## License

Apache-2.0 — see [LICENSE](LICENSE).
