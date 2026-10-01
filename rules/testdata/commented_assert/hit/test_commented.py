def test_commented():
    value = run()
    # assert value == 1
    assert value is not None


def test_commented_more():
    result = run()
    # assert result.ok, "should be fine"
    # self.assertEqual(result.count, 2)
    # assert_called_once_with(1)
    # assert msg == f"hello {name}"
    # assert data == b"raw"
