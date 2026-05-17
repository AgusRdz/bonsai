# OAuth Integration Architecture

## Objetivo

Eliminar la dependencia del CLI externo (gh, glab, bb) para operaciones de PR/issues,
usando tokens OAuth directamente contra las APIs REST de cada proveedor. Esto permite:

- Funcionar sin gh/glab/bb instalados
- Mas features (inline comments, issue creation, PR templates)
- Respuestas mas rapidas (sin fork de proceso)
- Reutilizacion del token para Jira/Linear/Trello en el futuro

---

## Estructura propuesta

```
auth/
  token.go        -- Token struct, almacenamiento cifrado en ~/.bonsai.tokens
  keychain.go     -- Wrappers para OS keychain (macOS/Linux/Windows)
  device_flow.go  -- GitHub/GitLab device flow (no requiere callback server)
  manager.go      -- TokenManager: Get/Set/Delete por host

pr/
  gh_api.go       -- ghAPIProvider: implementa Provider usando la GitHub REST API
  glab_api.go     -- glabAPIProvider: idem para GitLab
  bb_api.go       -- bbAPIProvider: idem para Bitbucket
  provider.go     -- Detect() prioriza API provider si hay token, fallback a CLI

cmd/
  auth.go         -- `bonsai auth login <provider>` y `bonsai auth status`
```

---

## Token storage

### Prioridad de almacenamiento

1. **OS Keychain** (preferido): seguro, no aparece en `ps` ni logs
   - macOS: `security add-generic-password` via `go-keychain`
   - Linux: `secret-service` via D-Bus / `libsecret`
   - Fallback: archivo cifrado

2. **Archivo cifrado** (~/.bonsai.tokens): AES-256-GCM, clave derivada del machine ID
   - Formato: TOML con tokens por host, cifrados en base64
   - `chmod 600` al crear

### Token struct

```go
package auth

type Token struct {
    Host        string    // "github.com", "gitlab.com", "bitbucket.org"
    AccessToken string
    TokenType   string    // "Bearer"
    Scopes      []string  // ["repo", "read:user", ...]
    ExpiresAt   time.Time // zero = no expiry
    RefreshedAt time.Time
}

type Manager interface {
    Get(host string) (*Token, error)
    Set(token Token) error
    Delete(host string) error
    List() ([]Token, error)
}
```

---

## Device Flow (sin callback server)

GitHub y GitLab soportan el OAuth Device Flow (RFC 8628), que no requiere
un servidor HTTP local para recibir el redirect. Flujo:

```
1. POST /login/device/code
   body: client_id=<app_id>&scope=repo,read:user
   response: { device_code, user_code, verification_uri, interval, expires_in }

2. Mostrar en TUI:
   "Abre https://github.com/login/device y escribe: ABCD-1234"
   [q] cancelar

3. Poll /login/oauth/access_token cada `interval` segundos hasta:
   - success: { access_token, token_type, scope }
   - authorization_pending: seguir polling
   - slow_down: aumentar interval
   - expired_token / access_denied: error

4. Guardar token via Manager.Set()
```

### OAuth App IDs (necesarios para deploy real)

| Proveedor | Crear en | Scope minimo |
|---|---|---|
| GitHub | Settings > Developer settings > OAuth Apps | `repo`, `read:user` |
| GitLab | User Settings > Applications | `api`, `read_user` |
| Bitbucket | Workspace > Apps > OAuth consumers | `pullrequests:write`, `issues:write` |

Para desarrollo, usar variables de entorno: `BONSAI_GITHUB_CLIENT_ID`, etc.

---

## gh_api.go - GitHub REST API provider

Implementa la misma interfaz `Provider` + todas las interfaces opcionales,
usando `net/http` directamente con el token OAuth:

```go
type ghAPIProvider struct {
    token  string
    client *http.Client
    base   string // "https://api.github.com"
}

// Ejemplo: ListPRs
func (g *ghAPIProvider) ListPRs(ctx context.Context) ([]PRStatus, error) {
    // GET /repos/{owner}/{repo}/pulls?state=open
    // detecta owner/repo desde git remote
    // mapea JSON a []PRStatus
}
```

**Ventajas sobre gh CLI:**
- Sin fork de proceso - latencia ~5x menor
- Puede hacer llamadas en paralelo (fetch PR + CI status en un solo render)
- Acceso a endpoints no expuestos por gh (PR templates, draft conversion, etc.)

---

## Deteccion y fallback

`pr/provider.go` Detect() con prioridad:

```go
func Detect(remoteURL string) Provider {
    host := ParseRemoteHost(remoteURL)

    // 1. Si hay token OAuth, usar API provider (no depende del CLI)
    if token, err := auth.DefaultManager.Get(host); err == nil {
        switch host {
        case "github.com":
            return newGHAPIProvider(token)
        case "gitlab.com":
            return newGLabAPIProvider(token)
        case "bitbucket.org":
            return newBBAPIProvider(token)
        }
    }

    // 2. Fallback a CLI provider (comportamiento actual)
    for _, p := range registry {
        if p.DetectRemote(remoteURL) {
            return p
        }
    }
    return nil
}
```

---

## CLI: bonsai auth

```
bonsai auth login github    -- inicia device flow, muestra user_code, guarda token
bonsai auth login gitlab    -- idem
bonsai auth login bitbucket -- idem (usa app password si no hay device flow)
bonsai auth status          -- lista tokens guardados con host, scopes, expiry
bonsai auth logout github   -- elimina token del keychain
```

Integrado en `main.go` como `case "auth": runAuth(os.Args[2:])`.

---

## Fases de implementacion

### Fase 1 (v0.55) - Infraestructura base
- [ ] `auth/token.go` - Token struct + archivo cifrado (sin keychain todavia)
- [ ] `auth/device_flow.go` - GitHub device flow completo
- [ ] `bonsai auth login github` / `bonsai auth status` / `bonsai auth logout`

### Fase 2 (v0.56) - GitHub API provider
- [ ] `pr/gh_api.go` - ListPRs, CurrentPR, CreatePR, Approve, RequestChanges
- [ ] `pr/gh_api.go` - ListIssues, CreateIssueBranch
- [ ] Detect() usa API provider cuando hay token

### Fase 3 (v0.57) - GitLab API provider
- [ ] `auth/device_flow.go` - GitLab device flow
- [ ] `pr/glab_api.go` - MR/Issues via API

### Fase 4 (v0.58) - OS Keychain + Bitbucket
- [ ] `auth/keychain.go` - macOS/Linux keychain
- [ ] `pr/bb_api.go` - Bitbucket API (app passwords, no device flow)
- [ ] Bitbucket OAuth 2.0 via implicit grant

### Fase 5 (futuro) - Integraciones PM
- [ ] Jira: OAuth 2.0 + mostrar issues linkados a branch en main panel
- [ ] Linear: API key (no OAuth) + mostrar issues
- [ ] GitHub Projects: GraphQL API
