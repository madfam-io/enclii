#!/usr/bin/env python3
"""Keep the verdaccio-auth-janua ConfigMap in sync with the plugin source.

The plugin ships to the Verdaccio pod as a ConfigMap (mounted at
/verdaccio/plugins/verdaccio-auth-janua), but the reviewable source of truth
lives in plugins/verdaccio-auth-janua/. Two copies drift; this script renders
one from the other.

  python3 scripts/sync-verdaccio-janua-plugin.py           # check (CI gate)
  python3 scripts/sync-verdaccio-janua-plugin.py --write    # regenerate

Exit 0 when in sync, 1 when drifted (or after --write when a change was made).
"""

import argparse
import pathlib
import sys

REPO_ROOT = pathlib.Path(__file__).resolve().parents[1]
PLUGIN_DIR = REPO_ROOT / "infra/k8s/base/verdaccio/plugins/verdaccio-auth-janua"
CONFIGMAP = REPO_ROOT / "infra/k8s/base/verdaccio/plugin-configmap.yaml"

# Files delivered into the pod, in ConfigMap key order.
DELIVERED = ("package.json", "index.js")

HEADER = """---
# ConfigMap that delivers the verdaccio-auth-janua plugin source into the pod.
# Mounted at /verdaccio/plugins/verdaccio-auth-janua/
#
# GENERATED FILE -- do not edit by hand.
# Source of truth: infra/k8s/base/verdaccio/plugins/verdaccio-auth-janua/
# Regenerate:      python3 scripts/sync-verdaccio-janua-plugin.py --write
# CI gate:         python3 scripts/sync-verdaccio-janua-plugin.py
apiVersion: v1
kind: ConfigMap
metadata:
  name: verdaccio-janua-plugin
  namespace: npm-registry
  labels:
    app.kubernetes.io/name: verdaccio
    app.kubernetes.io/component: auth-plugin
data:
"""


def indent_block(text: str, spaces: int = 4) -> str:
    """Render text as a YAML literal block scalar body.

    Blank lines stay truly blank so the block keeps a clean chomping shape.
    """
    pad = " " * spaces
    out = []
    for line in text.split("\n"):
        out.append(f"{pad}{line}" if line.strip() else "")
    return "\n".join(out)


def render() -> str:
    chunks = [HEADER]
    for name in DELIVERED:
        source = (PLUGIN_DIR / name).read_text()
        if source.endswith("\n"):
            source = source[:-1]
        chunks.append(f"  {name}: |\n{indent_block(source)}\n")
    return "".join(chunks)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--write", action="store_true", help="regenerate the ConfigMap in place"
    )
    args = parser.parse_args()

    for name in DELIVERED:
        path = PLUGIN_DIR / name
        if not path.is_file():
            print(f"ERROR missing plugin source: {path}", file=sys.stderr)
            return 1

    expected = render()
    current = CONFIGMAP.read_text() if CONFIGMAP.is_file() else ""

    if expected == current:
        print(f"OK    {CONFIGMAP.relative_to(REPO_ROOT)} matches plugin source")
        return 0

    if args.write:
        CONFIGMAP.write_text(expected)
        print(f"WROTE {CONFIGMAP.relative_to(REPO_ROOT)} from plugin source")
        return 0

    print(
        f"DRIFT {CONFIGMAP.relative_to(REPO_ROOT)} does not match "
        f"{PLUGIN_DIR.relative_to(REPO_ROOT)}\n"
        "      Run: python3 scripts/sync-verdaccio-janua-plugin.py --write",
        file=sys.stderr,
    )
    return 1


if __name__ == "__main__":
    sys.exit(main())
