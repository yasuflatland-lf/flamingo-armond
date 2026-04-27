"""Pure-logic tests for archive dict construction and UTC ISO8601 formatting (spec: destroy_env.md sections 4.6 and 5.5)."""

import datetime

import pytest

# ---------------------------------------------------------------------------
# Production logic (spec surrogate)
# ---------------------------------------------------------------------------

VALID_PHASES = frozenset({"vercel", "render", "supabase"})

# Fixed key set required in every archive record (section 4.6).
ARCHIVE_KEYS = frozenset({
    "teardown_at", "teardown_mode", "phase", "resource_id",
    "name", "team", "owner_email", "created_at",
    "delete_status", "http_status", "original_state",
})


def iso8601_utc(dt: datetime.datetime) -> str:
    """Return YYYY-MM-DDTHH:MM:SSZ.

    Sub-second digits are truncated (NOT rounded).  Suffix is always 'Z'.
    A naive datetime is treated as already-UTC (no offset conversion).
    """
    # replace(microsecond=0) truncates; strftime has no tz info so 'Z' is appended.
    return dt.replace(microsecond=0).strftime("%Y-%m-%dT%H:%M:%S") + "Z"


def build_archive(
    *,
    teardown_at: datetime.datetime,
    teardown_mode: str,
    phase: str,
    resource_id: str,
    identity: dict,
    delete_response: dict | None = None,
    original_state: object = None,
) -> dict:
    """Build the archive record dict matching the section 4.6 YAML schema.

    delete_status rules:
    - identity['already_gone'] True  -> 'already_gone'; http_status=None (key present)
    - delete_response status 204     -> 'deleted'
    - anything else                  -> 'failed'

    original_state defaults to None; the key is ALWAYS included (never omitted)
    so advisory-mode writes `original_state: null` per section 5.5.

    Raises ValueError for unknown phase.
    """
    if phase not in VALID_PHASES:
        raise ValueError(f"Unknown phase: {phase!r}. Must be one of {sorted(VALID_PHASES)}")

    if identity.get("already_gone"):
        delete_status, http_status = "already_gone", None
    else:
        status_code = (delete_response or {}).get("status", 0)
        delete_status = "deleted" if status_code == 204 else "failed"
        http_status = status_code if status_code else None

    return {
        "teardown_at": iso8601_utc(teardown_at),
        "teardown_mode": teardown_mode,
        "phase": phase,
        "resource_id": resource_id,
        "name": identity.get("name"),
        "team": identity.get("team"),
        "owner_email": identity.get("owner_email"),
        "created_at": identity.get("created_at"),
        "delete_status": delete_status,
        "http_status": http_status,
        "original_state": original_state,
    }


_SAMPLE_IDENTITY = {
    "name": "flamingo-armond",
    "team": "yasuflatland-team",
    "owner_email": "yasuflatland@gmail.com",
    "created_at": "2026-04-15T10:00:00Z",
}

_DT = datetime.datetime(2026, 4, 27, 15, 30, 18)


# ---------------------------------------------------------------------------
# Tests: iso8601_utc
# ---------------------------------------------------------------------------

def test_iso8601_zero_fractional():
    # Sub-second digits must be dropped (truncated), never rounded up.
    assert iso8601_utc(datetime.datetime(2026, 4, 27, 15, 30, 18, 123456)) == "2026-04-27T15:30:18Z"


def test_iso8601_uses_z_not_offset():
    result = iso8601_utc(_DT)
    assert result.endswith("Z")
    assert "+00:00" not in result


def test_iso8601_naive_treated_as_utc():
    # Naive datetime has no tzinfo and is treated as already-UTC.
    dt = datetime.datetime(2026, 4, 27, 0, 0, 0)
    assert dt.tzinfo is None
    assert iso8601_utc(dt) == "2026-04-27T00:00:00Z"


# ---------------------------------------------------------------------------
# Tests: build_archive
# ---------------------------------------------------------------------------

def test_archive_shape_full():
    record = build_archive(
        teardown_at=_DT, teardown_mode="strict", phase="vercel",
        resource_id="prj_6X7W3M", identity=_SAMPLE_IDENTITY,
        delete_response={"status": 204},
    )
    assert set(record.keys()) == ARCHIVE_KEYS


def test_archive_status_deleted():
    record = build_archive(
        teardown_at=_DT, teardown_mode="strict", phase="vercel",
        resource_id="prj_6X7W3M", identity=_SAMPLE_IDENTITY,
        delete_response={"status": 204},
    )
    assert record["delete_status"] == "deleted"
    assert record["http_status"] == 204


def test_archive_status_already_gone():
    # delete_status is 'already_gone'; http_status key must be present with value None.
    identity = dict(_SAMPLE_IDENTITY, already_gone=True)
    record = build_archive(
        teardown_at=_DT, teardown_mode="strict", phase="render",
        resource_id="svc_abc123", identity=identity,
    )
    assert record["delete_status"] == "already_gone"
    assert "http_status" in record
    assert record["http_status"] is None


def test_archive_status_failed():
    record = build_archive(
        teardown_at=_DT, teardown_mode="strict", phase="supabase",
        resource_id="ref_xyz", identity=_SAMPLE_IDENTITY,
        delete_response={"status": 503},
    )
    assert record["delete_status"] == "failed"


def test_archive_original_state_only_first():
    # Key must be present with value None when not supplied (section 5.5: null in advisory mode).
    record = build_archive(
        teardown_at=_DT, teardown_mode="advisory", phase="vercel",
        resource_id="prj_6X7W3M", identity=_SAMPLE_IDENTITY,
        delete_response={"status": 204},
    )
    assert "original_state" in record
    assert record["original_state"] is None


def test_archive_unknown_phase_raises():
    with pytest.raises(ValueError):
        build_archive(
            teardown_at=_DT, teardown_mode="strict", phase="paypal",
            resource_id="acct_999", identity=_SAMPLE_IDENTITY,
            delete_response={"status": 204},
        )
