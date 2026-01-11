# enclii logout

Clear local authentication credentials.

## Synopsis

```bash
enclii logout [flags]
```

## Description

The `logout` command removes stored authentication tokens from the local configuration file. This does not revoke the token server-side; use the web dashboard for token revocation.

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--all` | bool | `false` | Also clear project-specific tokens |

## Examples

### Standard Logout
```bash
enclii logout
# Output: Successfully logged out
```

### Clear All Tokens
```bash
enclii logout --all
# Clears main token and any project-specific API tokens
```

## What Gets Cleared

- Access token
- Refresh token
- Token expiration data
- Cached user information

## What Remains

- API URL configuration
- Default project/environment settings
- Output format preferences

## See Also

- [`enclii login`](./login.md) - Authenticate with Enclii
- [`enclii whoami`](./whoami.md) - Check current authentication
