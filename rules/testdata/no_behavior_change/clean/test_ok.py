def test_ok():
    obj = Foo()
    assert isinstance(obj, Foo)
    assert obj.value == 42
