def test_cache_hit_first():
    """First scenario: warm cache is used."""
    value = fetch("a")
    assert value


def test_something_unrelated():
    assert other() == 3


def test_something_else_unrelated():
    assert another() == 4


def test_cache_hit_second():
    """Second scenario: stale cache is refreshed."""
    value = fetch("b")
    assert value


def test_adjacent_documented_a():
    """Reads the primary replica."""
    value = read("primary")
    assert value


def test_adjacent_documented_b():
    """Reads the fallback replica."""
    value = read("fallback")
    assert value
