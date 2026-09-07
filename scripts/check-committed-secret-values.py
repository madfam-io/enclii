#!/usr/bin/env python3
"""Fail when a committed core/v1 Secret carries a real value.

WHY THIS EXISTS
===============
On 2026-09-06 `infra/k8s/base/verdaccio/secret.yaml` was found in this PUBLIC
repository: a plain `kind: Secret` whose `stringData.htpasswd` held a live
bcrypt hash, applied to the cluster by kustomize/ArgoCD. Nothing in CI objected,
because the existing hygiene guard is a line-oriented grep for known *token
shapes* (`sk_live_`, `ghp_`, `AKIA…`) and a bcrypt hash is none of them.

This checker closes that class: it parses YAML, finds Secret documents, and
asks whether the value sitting in `data:`/`stringData:` is credential material.

WHAT COUNTS AS A FINDING
========================
A Secret document is a finding when a `data`/`stringData` value survives the
placeholder filter AND looks like credential material — either it matches a
known credential shape (bcrypt/crypt hash, PEM private key, Stripe/GitHub/AWS/
Vault/Slack/Google key, JWT, npm token), or it contains a hex digest of 32+
characters, or it contains an opaque high-entropy run (a random-looking string
that is not a hostname, path, or version).

WHAT DOES NOT COUNT, AND WHY THERE IS NO ALLOWLIST
==================================================
Measured over this tree on 2026-09-07, committed Secrets hold 56 non-empty
values. Most are not secrets at all: `type: helm`, `enableOCI: "true"`,
`BILLING_MX_VAT_RATE: "0.16"`, `url: ghcr.io`, `database: enclii_dev`, and a
verbatim Prometheus scrape-config. A rule that failed on "any non-placeholder
value" would report ~20 non-findings, and the only way to get CI green again
would be an allowlist — which is exactly the mechanism that lets the next real
secret in, since an allowlist entry is cheap to add and never re-reviewed.

So the rule discriminates by the SHAPE OF THE VALUE, not by the name of its
key and not by a registry of blessed files. Two consequences, stated plainly
because a guard that implies more coverage than it has is worse than none:

  - A short, low-entropy secret (`password: hunter2`) is NOT caught. It is
    indistinguishable by shape from `type: helm`. Key-name heuristics were
    considered and rejected: `htpasswd` is not an obvious credential key name,
    and a key allowlist is an allowlist.
  - A value is judged on shape alone, so a genuinely random-looking non-secret
    would be a false positive. None exist in this tree today; if one appears,
    the fix is to move it out of a Secret, not to register an exception.

The target is zero committed Secret values, so the honest failure mode of this
guard is under-reporting, never a growing exception list.

USAGE
=====
    python3 scripts/check-committed-secret-values.py            # tracked files
    python3 scripts/check-committed-secret-values.py FILE...    # explicit set

Exit codes:
  0 — no committed Secret carries credential material
  1 — at least one does
  2 — UNDETERMINED (PyYAML missing, or the file set could not be read). Fails
      CI exactly like 1: a scan that parsed nothing proves nothing.
"""

from __future__ import annotations

import base64
import math
import re
import subprocess
import sys
from collections import Counter
from pathlib import Path

try:
    import yaml
except ImportError:  # pragma: no cover - exercised by the missing-dep path
    print(
        "error: PyYAML is required. Install with: pip install pyyaml",
        file=sys.stderr,
    )
    print("committed_secret_values=UNDETERMINED docs=0", file=sys.stderr)
    sys.exit(2)


# Kinds that are safe by construction: they carry a REFERENCE to a secret held
# elsewhere (a store, a sealed blob), never the plaintext value itself.
ALLOWED_SECRET_KINDS = {
    "ExternalSecret",
    "ClusterExternalSecret",
    "SealedSecret",
    "SecretStore",
    "ClusterSecretStore",
    "PushSecret",
    "VaultStaticSecret",
    "VaultDynamicSecret",
}

# Only core/v1 Secret is the object that actually holds material. A CRD that
# happens to be named Secret in another group is not this class.
CORE_SECRET_API_VERSIONS = {"v1", "core/v1"}

SECRET_VALUE_BLOCKS = ("data", "stringData")

# Placeholder vocabulary already used by this repo's Secret templates:
#   __CHANGE_ME_LOCAL_ONLY__      infra/k8s/base/secrets.dev.yaml
#   REPLACE_ME / REPLACE_BEFORE_APPLY / REPLACE_VIA_KUBECTL_CREATE_SECRET
#   <pk_live_xxx>, <vendor_id>    infra/k8s/production/billing/secrets-template.yaml
#   ${GITHUB_TOKEN}               infra/argocd/repo-credentials.yaml
#   __GENERATE_LOCAL_CERT_OUTSIDE_GIT__
# plus the forms the sibling grep guard already excludes (YOUR_, REDACTED, %s).
PLACEHOLDER = re.compile(
    r"""(
      \$\{[^}]*\}          # ${VAR}
    | \$\([^)]*\)          # $(cmd)
    | \{\{[^}]*\}\}        # {{ template }}
    | <[^>\s]*>            # <pk_live_xxx>, <REDACTED>
    | %[sdv]               # printf-style
    | CHANGE_?ME
    | REPLACE
    | REDACT
    | PLACEHOLDER
    | YOUR_
    | EXAMPLE
    | GENERATE_
    | OUTSIDE_GIT
    | NOT_SET
    | \bXXX+
    | \bTBD\b
    | \bTODO\b
    | \bDUMMY\b
    | \bFAKE\b
    | \bSAMPLE\b
    )""",
    re.IGNORECASE | re.VERBOSE,
)

# A placeholder rarely sits alone: `https://REPLACE_ME.r2.cloudflarestorage.com`
# is one token. Blanking only the marker leaves `_ME.r2.cloudflarestorage.com`,
# which reads as opaque. Blank the whole whitespace-delimited token instead.
PLACEHOLDER_TOKEN = re.compile(r"\S*(?:" + PLACEHOLDER.pattern + r")\S*", re.IGNORECASE | re.VERBOSE)

# Credential material with a recognisable shape. These are findings on sight,
# at any length and any entropy.
CREDENTIAL_SHAPES: list[tuple[re.Pattern[str], str]] = [
    (re.compile(r"\$2[abxy]?\$[0-9]{2}\$[./A-Za-z0-9]{53}"), "bcrypt hash"),
    (re.compile(r"\$(?:argon2[a-z]*|scrypt|[1256y])\$[^\s:$]{4,}\$[^\s:]{10,}"), "crypt/argon2 hash"),
    (re.compile(r"-----BEGIN [A-Z ]*PRIVATE KEY-----"), "PEM private key"),
    (re.compile(r"\b(?:sk|pk|rk)_(?:live|test)_[A-Za-z0-9]{10,}"), "Stripe key"),
    (re.compile(r"\bgh[pousr]_[A-Za-z0-9]{20,}"), "GitHub token"),
    (re.compile(r"\bAKIA[0-9A-Z]{16}\b"), "AWS access key id"),
    (re.compile(r"\bhvs\.[A-Za-z0-9_-]{15,}"), "Vault token"),
    (re.compile(r"\bey[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]+"), "JWT"),
    (re.compile(r"\bnpm_[A-Za-z0-9]{30,}"), "npm token"),
    (re.compile(r"\bxox[baprs]-[A-Za-z0-9-]{10,}"), "Slack token"),
    (re.compile(r"\bAIza[0-9A-Za-z_-]{30,}"), "Google API key"),
    (re.compile(r"\bSG\.[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}"), "SendGrid key"),
    (re.compile(r"\bglpat-[A-Za-z0-9_-]{15,}"), "GitLab token"),
]

# Opaque-run thresholds. Calibrated 2026-09-07 against every value committed in
# a Secret in this tree: the only value clearing them is the verdaccio bcrypt
# hash, and it is already caught by shape above.
OPAQUE_MIN_LEN = 24
OPAQUE_MIN_ENTROPY = 3.6
OPAQUE_CANDIDATE = re.compile(r"[A-Za-z0-9+/=_.\-]{%d,}" % OPAQUE_MIN_LEN)

# A single-case hex digest (md5/sha1/sha256, or a hex API key) never clears the
# three-character-class test below, because hex has no uppercase-and-lowercase
# mix and its entropy per character caps at 4.0. It is still credential
# material, so it gets its own rule: >=32 hex chars, not a git SHA context.
HEX_RUN = re.compile(r"(?<![A-Za-z0-9])[0-9a-f]{32,}(?![A-Za-z0-9])|(?<![A-Za-z0-9])[0-9A-F]{32,}(?![A-Za-z0-9])")

# Structured strings that are long and mixed-case but carry no secret: DNS
# names, filesystem paths, URLs without a userinfo component, container refs,
# and version strings. Checked against the candidate run, not the whole value.
STRUCTURED_RUN = re.compile(
    r"""^(?:
        (?:[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?\.)+[A-Za-z]{2,}\.?   # a.b.example.com
      | [/~][A-Za-z0-9._/\-]*                                            # /var/lib/thing
      | v?[0-9]+(?:\.[0-9]+)+(?:[-+][A-Za-z0-9.\-]+)?                    # 1.2.3-rc1
    )$""",
    re.VERBOSE,
)


def shannon_entropy(value: str) -> float:
    """Bits per character. A random base64 run sits near 5.0; English near 4.0."""
    if not value:
        return 0.0
    counts = Counter(value)
    length = len(value)
    return -sum((n / length) * math.log2(n / length) for n in counts.values())


def opaque_run(value: str) -> str | None:
    """Return the longest run that reads as random credential material."""
    best: str | None = None
    for match in OPAQUE_CANDIDATE.finditer(value):
        run = match.group(0)
        if STRUCTURED_RUN.match(run):
            continue
        # Require genuine character-class mixing. `enclii-production-database`
        # is long but lowercase-and-hyphen only; a key or hash is not.
        classes = sum(
            bool(re.search(pattern, run))
            for pattern in (r"[a-z]", r"[A-Z]", r"[0-9]")
        )
        if classes < 3:
            continue
        if shannon_entropy(run) < OPAQUE_MIN_ENTROPY:
            continue
        if best is None or len(run) > len(best):
            best = run
    return best


def decode_value(raw: object, block: str) -> str:
    """Render one Secret value as text; `data:` entries are base64."""
    text = "" if raw is None else str(raw)
    if block != "data":
        return text
    try:
        return base64.b64decode(text, validate=True).decode("utf-8", "replace")
    except Exception:
        # Not decodable base64 — judge the literal, which is what is committed.
        return text


def classify(value: str) -> str | None:
    """Return a reason when this value is credential material, else None."""
    if not value.strip():
        return None

    # Known credential shapes are judged against the RAW value, before any
    # placeholder blanking. A bcrypt hash is a bcrypt hash no matter what sits
    # next to it, and the token-level blanking below would otherwise erase a
    # real hash purely because its htpasswd line reads `someone@example.com:…`
    # — a placeholder word adjacent to live material must not launder it.
    for pattern, name in CREDENTIAL_SHAPES:
        if pattern.search(value):
            return name

    # The fuzzier heuristics DO need the blanking, or `${GITHUB_TOKEN}` and
    # `https://REPLACE_ME.r2.cloudflarestorage.com` read as opaque runs.
    residue = PLACEHOLDER_TOKEN.sub(" ", value)
    if not residue.strip():
        return None
    hex_match = HEX_RUN.search(residue)
    if hex_match is not None:
        return f"hex digest/key ({len(hex_match.group(0))} hex chars)"
    run = opaque_run(residue)
    if run is not None:
        return f"opaque high-entropy value ({len(run)} chars, {shannon_entropy(run):.1f} bits/char)"
    return None


def is_core_secret(doc: dict) -> bool:
    if doc.get("kind") != "Secret":
        return False
    api_version = str(doc.get("apiVersion") or "v1").strip()
    return api_version in CORE_SECRET_API_VERSIONS


def scan_document(path: str, doc: object, findings: list[str]) -> None:
    if not isinstance(doc, dict):
        return
    kind = doc.get("kind")
    if kind in ALLOWED_SECRET_KINDS:
        return
    if not is_core_secret(doc):
        return
    metadata = doc.get("metadata") if isinstance(doc.get("metadata"), dict) else {}
    name = metadata.get("name", "<unnamed>")
    for block in SECRET_VALUE_BLOCKS:
        entries = doc.get(block)
        if not isinstance(entries, dict):
            continue
        for key, raw in entries.items():
            reason = classify(decode_value(raw, block))
            if reason:
                findings.append(f"{path}: Secret/{name} {block}.{key} — {reason}")


def tracked_yaml_files() -> tuple[bool, list[str]]:
    """Return (listing succeeded, paths). An empty list from a successful
    listing means "this repo tracks no YAML", which is a clean answer — it is
    not the same as git failing, which is UNDETERMINED."""
    try:
        result = subprocess.run(
            ["git", "ls-files", "-z", "--", "*.yaml", "*.yml"],
            capture_output=True,
            text=True,
            check=False,
        )
    except OSError:
        return False, []
    if result.returncode != 0:
        return False, []
    return True, [p for p in result.stdout.split("\0") if p]


def main(argv: list[str]) -> int:
    if argv:
        files = argv
    else:
        ok, files = tracked_yaml_files()
        if not ok:
            # `git ls-files` itself failed (not a repo, git missing). That is
            # UNDETERMINED: we could not establish what is committed.
            print(
                "[committed-secret] UNDETERMINED — could not list tracked files",
                file=sys.stderr,
            )
            print("committed_secret_values=UNDETERMINED docs=0", file=sys.stderr)
            return 2
        # A repo that tracks no YAML at all commits no Secret, which is clean.
        # Only a failed listing is UNDETERMINED; an empty one is an answer.

    findings: list[str] = []
    docs_read = 0
    for path in files:
        try:
            text = Path(path).read_text(encoding="utf-8")
        except (OSError, UnicodeDecodeError):
            continue
        # Cheap prefilter: parsing every manifest in the tree is the slow path.
        if "Secret" not in text:
            continue
        try:
            documents = list(yaml.safe_load_all(text))
        except yaml.YAMLError:
            # Templated manifests (Helm, envsubst) are not valid YAML and are
            # not applied as-is. Unparseable is not a finding, but say so.
            print(f"[committed-secret] skipped unparseable YAML: {path}", file=sys.stderr)
            continue
        for doc in documents:
            docs_read += 1
            scan_document(path, doc, findings)

    if findings:
        print("\n[committed-secret] Secret with committed credential material", file=sys.stderr)
        for finding in findings:
            print(f"  {finding}", file=sys.stderr)
        print(
            "\nA core/v1 Secret in a public repo must carry no real value. Move it to an\n"
            "ExternalSecret (Vault-backed) or a SealedSecret, or reduce the manifest to a\n"
            "name-only Secret shell and apply the value out of band. Rotate the value: it\n"
            "is in git history, and deleting the line from HEAD does not remove it.",
            file=sys.stderr,
        )

    print(
        f"committed_secret_values={'FAIL' if findings else 'OK'} "
        f"docs={docs_read} findings={len(findings)}"
    )
    return 1 if findings else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
