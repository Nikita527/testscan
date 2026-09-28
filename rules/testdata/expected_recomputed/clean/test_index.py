def test_index_compare_ok():
    keys = ordered(plan)
    assert keys.index(A) < keys.index(B)
