import unittest

from df_app.service import process


def assert_incremented(value):
    assert value > 0


def test_helper_gets_result():
    result = process(1)
    assert_incremented(result)


def test_helper_prefix_check():
    check_positive(process(1))


class TestStyle(unittest.TestCase):
    def test_unittest(self):
        self.assertEqual(process(1), 2)

    def test_tuple_unpack_and_loop(self):
        a, b = process(1), process(2)
        total = 0
        for v in [a, b]:
            total += v
        assert total == 5

    def test_comprehension_fstring(self):
        rows = [process(i) for i in range(3)]
        msg = f"rows={rows}"
        assert "rows" in msg
