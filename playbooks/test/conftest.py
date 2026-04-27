"""Layer-3 pytest harness for teardown-prod _lib/*.yml include tasks.

Design
------
Each test stands up a real local HTTP server via pytest-httpserver (bound to
127.0.0.1 with an OS-assigned port), then invokes `ansible-playbook` as a
subprocess against a thin test entry playbook in playbooks/test/. The test
entry uses `include_tasks` to pull in the _lib file under test. Assertions
are made against the process return code, stdout/stderr text, and YAML files
written to a tmp_path archive directory.

Dependencies
------------
  pytest
  pytest-httpserver   (werkzeug-based; gives Ansible's uri module a real socket)
  pyyaml

Install:
  pip install --user pytest pytest-httpserver pyyaml

These tests are optional in current CI. They document expected behavior of
the _lib/*.yml includes and can be run manually with:
    pip install --user pytest pytest-httpserver pyyaml
    pytest playbooks/test/
"""

import json
import os
import subprocess
from pathlib import Path

import pytest

# Optional imports — the modifyitems hook skips all tests when missing.
try:
    from pytest_httpserver import HTTPServer as _HTTPServer  # noqa: F401
    _httpserver_available = True
except ImportError:
    _httpserver_available = False

try:
    import yaml
    _yaml_available = True
except ImportError:
    _yaml_available = False

# ---------------------------------------------------------------------------
# Repository root
# ---------------------------------------------------------------------------

def repo_root() -> Path:
    """Return the repository root, resolved from this file's location."""
    return Path(__file__).resolve().parents[2]


# ---------------------------------------------------------------------------
# Collection hook: skip the whole directory if optional deps are missing
# ---------------------------------------------------------------------------

def pytest_collection_modifyitems(items):
    """Auto-skip layer-3 tests when pytest-httpserver or pyyaml is absent."""
    missing = []
    if not _httpserver_available:
        missing.append("pytest-httpserver")
    if not _yaml_available:
        missing.append("pyyaml")

    if not missing:
        return

    reason = (
        "Layer-3 integration tests require optional dependencies that are not "
        "installed: {}. Run: pip install --user pytest pytest-httpserver pyyaml"
    ).format(", ".join(missing))

    skip_mark = pytest.mark.skip(reason=reason)
    for item in items:
        # Only skip tests defined inside this directory.
        if Path(item.fspath).parent == Path(__file__).parent:
            item.add_marker(skip_mark)


# ---------------------------------------------------------------------------
# Fixture: override httpserver bind address (port 0 = OS picks a free port)
# ---------------------------------------------------------------------------

@pytest.fixture(scope="function")
def httpserver_listen_address():
    """Bind the mock HTTP server to 127.0.0.1 on an OS-assigned free port."""
    return ("127.0.0.1", 0)


# ---------------------------------------------------------------------------
# Fixture: archive_dir
# ---------------------------------------------------------------------------

@pytest.fixture()
def archive_dir(tmp_path) -> Path:
    """Return a pre-created archive directory under tmp_path (mode 0700)."""
    d = tmp_path / "archive"
    d.mkdir(mode=0o700, parents=True, exist_ok=True)
    return d


# ---------------------------------------------------------------------------
# Fixture: run_playbook
# ---------------------------------------------------------------------------

@pytest.fixture()
def run_playbook(archive_dir):
    """Return a callable that runs an Ansible test playbook as a subprocess.

    Signature:
        run_playbook(test_yml, *, extravars, env=None) -> CompletedProcess

    Parameters
    ----------
    test_yml:
        Filename (not path) of the playbook under playbooks/test/.
    extravars:
        Dict of extra variables passed via -e key=value pairs.
    env:
        Optional dict merged on top of os.environ for the subprocess.
        Use to inject API tokens via environment variables.

    The call always adds:
        -e testing=true
        -e archive_dir=<archive_dir>
    plus one -e key=value for each entry in extravars.

    Returns the CompletedProcess; does NOT raise on non-zero exit so tests
    can assert on returncode directly.
    """
    root = repo_root()
    inventory = str(root / "playbooks" / "inventory.local")

    def _run(
        test_yml: str,
        *,
        extravars: dict,
        env: "dict | None" = None,
    ) -> subprocess.CompletedProcess:
        cmd = [
            "ansible-playbook",
            "-i", inventory,
            str(root / "playbooks" / "test" / test_yml),
            "-e", "testing=true",
            "-e", f"archive_dir={archive_dir}",
        ]
        for key, value in extravars.items():
            cmd += ["-e", f"{key}={value}"]

        merged_env = {**os.environ}
        if env:
            merged_env.update(env)

        return subprocess.run(
            cmd,
            check=False,
            capture_output=True,
            text=True,
            cwd=str(root),
            env=merged_env,
        )

    return _run


# ---------------------------------------------------------------------------
# Fixture: provider_get_fixture
# ---------------------------------------------------------------------------

@pytest.fixture()
def provider_get_fixture():
    """Return a callable that loads a provider GET fixture JSON file.

    Usage:
        data = provider_get_fixture("vercel")
        # loads playbooks/test/fixtures/vercel_*_get.json

    Returns the parsed dict. Raises FileNotFoundError if no matching file
    exists.
    """
    fixtures_dir = Path(__file__).parent / "fixtures"

    def _load(provider: str) -> dict:
        matches = sorted(fixtures_dir.glob(f"{provider}_*_get.json"))
        if not matches:
            raise FileNotFoundError(
                f"No fixture file matching '{provider}_*_get.json' in {fixtures_dir}"
            )
        with matches[0].open() as fh:
            return json.load(fh)

    return _load


# ---------------------------------------------------------------------------
# Fixture: archive_files
# ---------------------------------------------------------------------------

@pytest.fixture()
def archive_files(archive_dir):
    """Return a callable that reads and parses all .yml files in archive_dir.

    Usage:
        records = archive_files()  # -> list[dict], sorted by filename

    Each file is parsed with yaml.safe_load. Files that contain a mapping are
    returned as dicts; other documents are returned as-is.
    """
    def _read() -> list:
        yml_files = sorted(archive_dir.glob("*.yml"))
        results = []
        for path in yml_files:
            with path.open() as fh:
                results.append(yaml.safe_load(fh))
        return results

    return _read
