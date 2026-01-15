# enclii login

Authenticate with Enclii using SSO (Single Sign-On).

## Synopsis

```bash
enclii login [flags]
```

## Description

The `login` command initiates authentication with Enclii via the configured SSO provider (Janua). By default, it opens a browser for interactive login. For CI/CD environments, use the `--token` flag or set `ENCLII_TOKEN`.

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--sso` | bool | `true` | Use SSO authentication (browser-based) |
| `--token` | string | | API token for non-interactive authentication |
| `--no-browser` | bool | `false` | Print login URL instead of opening browser |

## Examples

### Interactive Login (Default)
```bash
enclii login
# Opens browser for SSO authentication
```

### CI/CD Authentication
```bash
# Using environment variable (recommended)
export ENCLII_TOKEN=your-api-token
enclii whoami

# Using flag
enclii login --token your-api-token
```

### Headless Environment
```bash
enclii login --no-browser
# Prints: Visit https://auth.madfam.io/authorize?...
# Copy URL to browser on another machine
```

## Authentication Flow

1. CLI initiates OAuth 2.0 PKCE flow
2. Browser opens to Janua SSO (`auth.madfam.io`)
3. User authenticates (email/password, OAuth, or passkey)
4. Janua redirects to local callback server
5. CLI exchanges code for JWT tokens
6. Tokens stored in `~/.enclii/config.yaml`

## Token Storage

Credentials are stored in:
- **macOS**: `~/.enclii/config.yaml` (file permissions: 600)
- **Linux**: `~/.enclii/config.yaml` (file permissions: 600)
- **Windows**: `%USERPROFILE%\.enclii\config.yaml`

## Security Notes

- Tokens are automatically refreshed before expiration
- Use `enclii logout` to clear stored credentials
- For CI/CD, use short-lived API tokens from the dashboard
- Never commit tokens to version control

## See Also

- [`enclii logout`](./logout.md) - Clear credentials
- [`enclii whoami`](./whoami.md) - Verify authentication
- [SSO Integration Guide](../../integrations/sso.md)
