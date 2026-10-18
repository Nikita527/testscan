def ComputeExpected(x):
    return x * 2


def test_camelcase_helper_recomputed():
    got = ComputeExpected(3)
    assert got == ComputeExpected(3)
