import pytest


@pytest.mark.slow
def test_parse_big_payload():
    doc = parse("big")
    assert doc


def test_parse_small_payload():
    doc = parse("small")
    assert doc
