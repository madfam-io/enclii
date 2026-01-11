# enclii whoami

Display information about the currently authenticated user.

## Synopsis

```bash
enclii whoami [flags]
```

## Description

The `whoami` command displays details about the currently authenticated user, including their email, user ID, and team memberships. Useful for verifying authentication and debugging access issues.

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--output`, `-o` | string | `table` | Output format: `table`, `json`, `yaml` |

## Examples

### Basic Usage
```bash
enclii whoami
```

**Output:**
```
Email:    developer@example.com
User ID:  usr_abc123def456
Teams:    acme-corp (admin), side-project (member)
Expires:  2025-01-12T15:30:00Z
```

### JSON Output
```bash
enclii whoami -o json
```

**Output:**
```json
{
  "email": "developer@example.com",
  "user_id": "usr_abc123def456",
  "teams": [
    {"name": "acme-corp", "role": "admin"},
    {"name": "side-project", "role": "member"}
  ],
  "token_expires_at": "2025-01-12T15:30:00Z"
}
```

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Authenticated successfully |
| `50` | Not authenticated or token expired |

## See Also

- [`enclii login`](./login.md) - Authenticate with Enclii
- [`enclii logout`](./logout.md) - Clear credentials
