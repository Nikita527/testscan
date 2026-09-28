def test_factory_expected():
    got = compute(3)
    assert got == make_user(3)
