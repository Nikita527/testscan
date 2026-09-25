from pkg.app import greet


def test_greet_ok():
    assert greet("a") == "hi a"
