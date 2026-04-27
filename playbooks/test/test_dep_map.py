"""Pure-logic tests for the reverse-dependency map."""

import pytest

# ---------------------------------------------------------------------------
# Production logic (spec surrogate)
# ---------------------------------------------------------------------------
# Upstream dependency declaration: a phase is blocked if any of its upstream
# phases are still alive (i.e., not yet torn down).  Declared order is
# preserved so callers can reason about teardown sequence deterministically.
DEP_MAP: dict[str, list[str]] = {
    "vercel": [],
    "render": ["vercel"],
    "supabase": ["vercel", "render"],
}

VALID_PHASES = frozenset(DEP_MAP)


def upstream_alive(phase: str, alive_set: set[str]) -> list[str]:
    """Return the subset of upstream phases that are still alive, in declared order.

    Args:
        phase:     One of the three known phases (vercel, render, supabase).
        alive_set: Set of phase names that have NOT yet been torn down.

    Returns:
        Ordered list of upstream phases present in alive_set.

    Raises:
        KeyError: phase is not in DEP_MAP.
    """
    if phase not in DEP_MAP:
        raise KeyError(f"Unknown phase: {phase!r}")
    return [p for p in DEP_MAP[phase] if p in alive_set]


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------

def test_dep_map_keys():
    assert set(DEP_MAP.keys()) == {"vercel", "render", "supabase"}


def test_vercel_no_upstream():
    # vercel has no upstream regardless of what is alive
    assert upstream_alive("vercel", {"vercel", "render", "supabase"}) == []
    assert upstream_alive("vercel", set()) == []


def test_render_blocked_by_vercel():
    # render is blocked iff vercel is still alive
    assert upstream_alive("render", {"vercel"}) == ["vercel"]
    assert upstream_alive("render", set()) == []
    assert upstream_alive("render", {"supabase"}) == []


def test_supabase_blocked_by_either():
    # supabase lists vercel and/or render based on alive_set; order is vercel -> render
    assert upstream_alive("supabase", {"vercel", "render"}) == ["vercel", "render"]
    assert upstream_alive("supabase", {"vercel"}) == ["vercel"]
    assert upstream_alive("supabase", {"render"}) == ["render"]
    assert upstream_alive("supabase", set()) == []
    # order is preserved: vercel before render
    result = upstream_alive("supabase", {"render", "vercel"})
    assert result.index("vercel") < result.index("render")


def test_unknown_phase_raises():
    with pytest.raises(KeyError):
        upstream_alive("paypal", {"vercel"})
    with pytest.raises(KeyError):
        upstream_alive("", set())
