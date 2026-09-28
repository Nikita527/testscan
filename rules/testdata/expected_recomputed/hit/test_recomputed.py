def test_recomputed():
    got = compute(3)
    assert got == compute(3)
