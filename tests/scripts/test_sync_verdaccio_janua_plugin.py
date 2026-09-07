"""
Tests for scripts/sync-verdaccio-janua-plugin.py.

Run with:
    pytest tests/scripts/test_sync_verdaccio_janua_plugin.py -v

The plugin exists twice on purpose: a reviewable source tree, and a ConfigMap
that actually reaches the pod. This gate is the only thing keeping the copy
that runs in production identical to the copy humans review.
"""
from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path

import yaml

REPO_ROOT = Path(__file__).resolve().parents[2]
SCRIPT = REPO_ROOT / "scripts" / "sync-verdaccio-janua-plugin.py"
PLUGIN_DIR = REPO_ROOT / "infra/k8s/base/verdaccio/plugins/verdaccio-auth-janua"
CONFIGMAP = REPO_ROOT / "infra/k8s/base/verdaccio/plugin-configmap.yaml"


def run_script(*args: str) -> tuple[int, str, str]:
    proc = subprocess.run(
        [sys.executable, str(SCRIPT), *args],
        capture_output=True,
        text=True,
    )
    return proc.returncode, proc.stdout, proc.stderr


def test_repo_is_in_sync() -> None:
    """The committed ConfigMap must already match the plugin source."""
    code, out, err = run_script()
    assert code == 0, f"ConfigMap drifted from plugin source:\n{out}\n{err}"


def test_configmap_is_valid_yaml_with_expected_shape() -> None:
    doc = yaml.safe_load(CONFIGMAP.read_text())
    assert doc["kind"] == "ConfigMap"
    assert doc["metadata"]["name"] == "verdaccio-janua-plugin"
    assert doc["metadata"]["namespace"] == "npm-registry"
    assert set(doc["data"]) == {"package.json", "index.js"}


def test_embedded_sources_are_byte_identical_to_the_plugin_tree() -> None:
    """A YAML literal block must round-trip the sources exactly.

    Indentation or chomping mistakes here would ship subtly different code to
    the pod than the code under review.
    """
    doc = yaml.safe_load(CONFIGMAP.read_text())
    for name in ("package.json", "index.js"):
        assert doc["data"][name] == (PLUGIN_DIR / name).read_text(), (
            f"{name} in the ConfigMap differs from the plugin source"
        )


def test_embedded_package_json_declares_no_dependencies() -> None:
    """The ConfigMap has no node_modules, so the plugin must be self-contained."""
    doc = yaml.safe_load(CONFIGMAP.read_text())
    pkg = json.loads(doc["data"]["package.json"])
    assert pkg["name"] == "verdaccio-auth-janua"
    assert pkg["main"] == "index.js"
    assert not pkg.get("dependencies"), "plugin must have no unbundled dependencies"


def test_detects_drift(tmp_path: Path, monkeypatch) -> None:
    """Mutating the source without regenerating must fail the gate."""
    index = PLUGIN_DIR / "index.js"
    original = index.read_text()
    try:
        index.write_text(original + "\n// drift\n")
        code, _out, err = run_script()
        assert code == 1
        assert "DRIFT" in err
    finally:
        index.write_text(original)

    # Restored: back in sync.
    assert run_script()[0] == 0


def test_write_regenerates_the_configmap() -> None:
    original = CONFIGMAP.read_text()
    try:
        CONFIGMAP.write_text("---\n# clobbered\n")
        assert run_script()[0] == 1
        code, out, _err = run_script("--write")
        assert code == 0
        assert "WROTE" in out
        assert CONFIGMAP.read_text() == original
    finally:
        CONFIGMAP.write_text(original)
