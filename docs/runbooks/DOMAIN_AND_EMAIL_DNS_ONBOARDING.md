---
title: Domain and email DNS onboarding
description: The verified operator sequence for bringing a brand host and a mail provider onto a zone Enclii hosts
tags: [runbook, dns, cloudflare, domains, email, tenancy]
---

# Domain and email DNS onboarding

> **Boundary checkpoint (2026-09-07, platform on-call):** Public-safe runbook.
> The domain and mail-provider hostnames here are the client's own public DNS
> and the providers' documented public endpoints; no node identity, no
> credential, and no tunnel identifier appears. Per-domain values an operator
> must supply — the Proton DKIM `<hash>`, verification tokens, the tunnel CNAME,
> `enclii-verification=<id>` — are placeholders, not values. Private
> operational detail and the onboarding sink live in `internal-devops`
> (2026-09-07 CTM tenant onboarding). Policy:
> `docs/PUBLIC_REPO_BOUNDARY.md` (repo-boundary contract).

What an operator actually has to do — and what currently bites — to bring a new
host, and a mail provider's record set, onto a zone Enclii hosts in Cloudflare.

Every step below was executed against `creatumundo.mx` on **2026-09-07** during
the CTM tenant onboarding. The gaps in
[Known gaps in `dns-apply`](#known-gaps-in-dns-apply) are open bugs
([#530](https://github.com/madfam-org/enclii/issues/530)), not preferences: read
them before you plan a record set, because two of them silently destroy records
you already created.

## Known gaps in `dns-apply`

### A record is keyed by name + type, so a second record at that name REPLACES the first

`dns-apply` reads the live record with
`GetDNSRecordByTypeInZone(zoneID, name, type)` and plans `create` when that read
returns nothing, `update` when it returns a record whose content differs. There
is no notion of "another record of this type at this name". So:

```bash
# 1. creates the Proton ownership TXT
enclii providers cloudflare dns-apply creatumundo.mx \
  --type TXT --content 'protonmail-verification=<token>' \
  --apply --reason "prove domain ownership to Proton"
# → created

# 2. DESTROYS it. The plan says `create`; the apply says `updated`.
enclii providers cloudflare dns-apply creatumundo.mx \
  --type TXT --content 'v=spf1 include:_spf.protonmail.ch ~all' \
  --apply --reason "publish SPF"
# → updated   ← the verification TXT is gone
```

This is not theoretical: it happened, and Proton loses the domain at its next
ownership re-check. The same limitation means the standard Proton MX **pair**
(priority 10 and 20 at the apex) cannot be expressed at all — the second apply
overwrites the first.

:::danger Break-glass until #530 lands

**Multiple records of the same type at one name must be added in the Cloudflare
dashboard by hand.** That is the exact manual DNS edit the Enclii-first doctrine
exists to prevent, and it is temporary. It applies to:

- more than one apex `TXT` (SPF *plus* any provider verification token — Proton,
  Google, Resend, Atlassian…), and
- more than one `MX` at any name (every real mail provider ships a pair).

A single record of a type at a name — the overwhelmingly common case, including
every `CNAME` — is safe through `dns-apply` and should go through it.

After a dashboard edit, re-read the authoritative state before you trust it:

```bash
dig +short TXT creatumundo.mx @<one of the zone's Cloudflare nameservers>
```

:::

### `--type MX --apply` can 502 while the dry-run passes

Two consecutive MX applies answered `502 origin_bad_gateway` from
`api.enclii.dev` while the dry-run for the same operation planned cleanly; the
same calls succeeded about 60 seconds later. If an MX apply 502s, **re-read the
zone before retrying** — a 502 is returned by the edge and does not tell you
whether the origin committed the write.

### MX priority rides inside `--content`

There is no `--priority` flag. The priority is the first token of the content
string:

```bash
enclii providers cloudflare dns-apply creatumundo.mx \
  --type MX --content '10 mail.protonmail.ch' \
  --apply --reason "primary MX for Proton Mail"
```

## Worked example: Proton Mail on a zone Enclii hosts

The full record set Proton requires. Each row notes whether `dns-apply` can
carry it today.

| Name | Type | Content | Via |
|------|------|---------|-----|
| `@` | TXT | `protonmail-verification=<token>` | dashboard — collides with SPF |
| `@` | MX | `10 mail.protonmail.ch` | dashboard — MX pair collides |
| `@` | MX | `20 mailsec.protonmail.ch` | dashboard — MX pair collides |
| `@` | TXT | `v=spf1 include:_spf.protonmail.ch ~all` | dashboard — collides with verification TXT |
| `protonmail._domainkey` | CNAME | `protonmail.domainkey.<hash>.domains.proton.ch` | `dns-apply` |
| `protonmail2._domainkey` | CNAME | `protonmail2.domainkey.<hash>.domains.proton.ch` | `dns-apply` |
| `protonmail3._domainkey` | CNAME | `protonmail3.domainkey.<hash>.domains.proton.ch` | `dns-apply` |
| `_dmarc` | TXT | `v=DMARC1; p=none; rua=mailto:<address>` | `dns-apply` — only TXT at that name |

`<hash>` is per-domain and shown in the Proton admin panel; it is not derivable.

The three DKIM CNAMEs and `_dmarc` are each the only record of their type at
their name, so they go through Enclii:

```bash
for n in "" 2 3; do
  enclii providers cloudflare dns-apply "protonmail${n}._domainkey.creatumundo.mx" \
    --type CNAME --content "protonmail${n}.domainkey.<hash>.domains.proton.ch" \
    --proxied false \
    --apply --reason "Proton Mail DKIM key ${n:-1}"
done
```

:::warning DKIM CNAMEs must not be proxied

A proxied record answers with Cloudflare's own addresses, and the DKIM lookup
gets an address instead of the delegation it needs. Pass `--proxied false`
explicitly: `dns-apply` defaults `A`/`AAAA`/`CNAME` to proxied.

:::

### Coexisting with Resend

A domain that already sends transactional mail through Resend keeps those
records; Proton and Resend do not conflict, because they occupy different names:

| Name | Type | Owner |
|------|------|-------|
| `resend._domainkey` | TXT | Resend DKIM |
| `send` | MX | Resend bounce handling |
| `send` | TXT | Resend SPF, scoped to the `send` subdomain |

The one place they *do* meet is the apex SPF. Resend's own records live under
`send.<domain>`, so the apex `v=spf1 include:_spf.protonmail.ch ~all` above is
correct as written and does not need a Resend `include`. Verify with
`dig +short TXT send.<domain>` that the Resend records survived any apex edit —
they are a different name, so they should, but the apex TXT collision above is
exactly the class of bug that makes checking worthwhile.

## `zone-settings-apply`: the HTTPS posture step

Run this **after the zone goes active** (that is, after the registrar
delegation lands and Cloudflare stops reporting the zone as `pending`). A
pending zone has no settings to read.

```bash
enclii providers cloudflare zone-settings-apply creatumundo.mx \
  --apply --reason "apply Enclii HTTPS posture to the client apex"
```

It applies three settings as a set, because they are only meaningful together —
`always_use_https` with a TLS floor of 1.0 still leaves the connection
downgradeable while reading as secure in a browser:

| Setting | Desired | Why |
|---------|---------|-----|
| `always_use_https` | `on` | redirect plain HTTP at the edge, before it reaches the origin |
| `automatic_https_rewrites` | `on` | rewrite `http://` subresources so a page does not mix content |
| `min_tls_version` | `1.2` | TLS 1.0/1.1 are deprecated and still offered by default |

Verify from outside — a 301 is the whole point of the step:

```bash
curl -sSI http://creatumundo.mx/ | head -1
# HTTP/1.1 301 Moved Permanently
```

If the dry-run reports `zone_absent`, the zone was never created: run
`providers cloudflare zone-add-apply` first. If a setting reports
`not-editable`, that is a zone-plan limitation, not a failure of this command.

## Domain onboarding sequence for a brand host

Verified end to end on 2026-09-07 for a host on a zone MADFAM hosts.

### 1. `domains add` — **from the repo root**

```bash
cd <repo root>            # NOT optional; see below
enclii domains add crea-erp.creatumundo.mx \
  --service nauta-web --env production -f enclii.yaml
```

:::warning `-f` does not change the working directory

The manifest parser resolves manifest-relative paths — `spec.build.dockerfile`
above all — against **`os.Getwd()`**, not against the directory the `-f` file
lives in (`packages/cli/internal/spec/parser.go`, `projectDir, err :=
os.Getwd()`). Running this from a subdirectory fails validation with
`spec.build.dockerfile: file does not exist: <path>` for a Dockerfile that is
plainly there. Run it from the repo root.

:::

`domains add` prints the CNAME target and the
`enclii-verification=<id>` TXT value used in steps 2 and 4.

### 2. `dns-apply` — proxied CNAME to the tunnel

```bash
enclii providers cloudflare dns-apply crea-erp.creatumundo.mx \
  --type CNAME --content <TUNNEL_CNAME> --proxied true \
  --apply --reason "route the new brand host through the Enclii tunnel"
```

### 3. `tunnels-apply` — reconcile the tunnel ingress

```bash
enclii providers cloudflare tunnels-apply --project <project> \
  --apply --reason "publish the tunnel route for the new brand host"
```

**Read the plan before applying.** It must show *only* a `create` for the new
hostname, plus skips for everything already live. Any `update` or `delete`
against a hostname you did not just add means the plan is about to rewrite a
working route — stop and investigate. (This guard exists because a legacy
manifest's declared domains once rewrote live routes to a dead backend.)

### 4. `dns-apply` — the verification TXT

```bash
enclii providers cloudflare dns-apply crea-erp.creatumundo.mx \
  --type TXT --content 'enclii-verification=<id>' \
  --apply --reason "prove ownership of the new brand host to Enclii"
```

Safe through `dns-apply`: it is the only TXT at that name. If the host already
carries another TXT, see the name+type collision above.

### 5. `domains verify`

```bash
enclii domains verify crea-erp.creatumundo.mx --service nauta-web
```

TLS is issued automatically once verification succeeds.

### Interaction with manifest capture

Deploy-time manifest capture is **idempotent with this sequence** — a later
deploy that declares the same host does not fight the records created above.

But a **manifest-only merge builds nothing**: when `build-publish` detects no
changed service, no image is built, so no deploy runs and therefore no capture
happens. A host added to `enclii.yaml` in a docs- or manifest-only PR does not
become live on merge. Either run the sequence above, or use
`enclii domains reconcile <service>` to provision the declared hostname
server-side.

## Resolver caveat after a nameserver switch

For up to the **old** zone's NS TTL after a registrar delegation change, clients
whose resolver still holds the previous nameservers resolve against the old
zone. During that window, on those clients only:

- a host that exists only in the new zone answers **NXDOMAIN**; and
- a DNS-only host that the old zone pointed at a Cloudflare-hosted CDN answers
  **Cloudflare error 1034**.

Both look exactly like a broken record you just wrote. They are not — the
authoritative check is against the new zone's own nameservers:

```bash
dig +short crea-erp.creatumundo.mx @<one of the zone's Cloudflare nameservers>
dig +short NS creatumundo.mx @1.1.1.1
dig +short NS creatumundo.mx @8.8.8.8
```

**Do not cut over redirects, and do not "fix" a record that already reads
correctly at the authoritative nameservers, until the public resolvers agree.**
Re-applying a correct record because a stale resolver disagreed is how a working
zone gets churned.

## CLI gotchas

### `enclii whoami` prints on stderr

`whoami`, `login`, and `logout` report through cobra's `cmd.Println`, which
writes to `OutOrStderr()`. The CLI never calls `SetOut`, so **all of that output
is on stderr**. `enclii whoami > /tmp/who` captures an empty file and reads as
"not logged in". Redirect with `2>&1`, or use `-o json`.

### The released CLI predates per-tenant Porkbun

`--tenant` and `providers porkbun ping` landed in
[#527](https://github.com/madfam-org/enclii/pull/527), which is **not** in
`v1.0.0-alpha.8` — that tag was cut from the commit immediately before it. On
alpha.8 the flag is rejected as unknown and `ping` does not exist. Build from
`main`:

```bash
go build -o ~/bin/enclii ./packages/cli/cmd/enclii
enclii providers porkbun ping --tenant crea
```

The next release tag — **`v1.0.0-alpha.9`** — is the one that should carry
these verbs.

### `enclii login` follows the browser's Janua session

`login` completes an OAuth PKCE flow in whatever browser session is already
authenticated at `auth.madfam.io`. Estate cookie precedence (janua J9) means a
browser logged into a **client** application resolves that identity, and the
CLI silently receives the wrong one — every subsequent `--tenant` call then
fails on authorization rather than on anything to do with the tenant.

**Log out of the client app in the browser before running `enclii login`**, and
confirm with `enclii whoami 2>&1` before you trust a session.

## Related

- [Porkbun per-tenant registrar credentials](/infrastructure/porkbun-tenant-credentials)
- [`enclii providers`](/cli/commands/providers)
- [`enclii domains`](/cli/commands/domains)
- [Cloudflare Integration](/infrastructure/CLOUDFLARE)
