"""Pure-logic tests for case-sensitive name matching (spec: destroy_env.md section 4.4)."""

import unicodedata

# ---------------------------------------------------------------------------
# Production logic (spec surrogate)
# ---------------------------------------------------------------------------
# The operator is prompted to retype the resource name.  The assertion is a
# byte-for-byte (character-for-character) equality check: no case folding, no
# Unicode normalization, no whitespace stripping.


def match_name(typed: str, expected: str) -> bool:
    """Return True iff typed equals expected with NO normalization of any kind.

    Deliberate non-normalizations:
    - Case differences are preserved (upper != lower).
    - Leading/trailing whitespace is preserved.
    - Unicode normalization forms (NFC vs NFD) are NOT equated.

    Args:
        typed:    The string the operator entered.
        expected: The authoritative name fetched from the provider API.

    Returns:
        True only when the two strings are identical code-point by code-point.
    """
    return typed == expected


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------

def test_exact_match_passes():
    assert match_name("flamingo-armond", "flamingo-armond") is True


def test_case_difference_fails():
    assert match_name("flamingo-armond", "Flamingo-Armond") is False
    assert match_name("FLAMINGO-ARMOND", "flamingo-armond") is False


def test_whitespace_difference_fails():
    assert match_name(" flamingo-armond", "flamingo-armond") is False
    assert match_name("flamingo-armond ", "flamingo-armond") is False
    assert match_name("flamingo-armond", "flamingo-armond ") is False


def test_unicode_normalization_disabled():
    # NFC form: single precomposed character U+00E9 (e + combining acute = e-acute)
    nfc = unicodedata.normalize("NFC", "é")
    # NFD form: base 'e' + U+0301 combining acute accent (two code points)
    nfd = unicodedata.normalize("NFD", "é")
    # The two forms must NOT be silently equated; match_name is intentionally strict
    assert nfc != nfd, "Precondition: NFC and NFD must differ as Python str objects"
    assert match_name(nfd, nfc) is False
    assert match_name(nfc, nfd) is False


def test_empty_typed_fails():
    assert match_name("", "flamingo-armond") is False
    # Two empty strings are technically identical, but the real gate is against
    # a non-empty provider name, so this edge case always fails in practice.
    # We only guarantee that empty typed != non-empty expected.
    assert match_name("", "x") is False
