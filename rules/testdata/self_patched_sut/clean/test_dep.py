from unittest.mock import patch


@patch("mod.dependency", return_value=7)
def test_dep_patch():
    assert mod.compute() == 7
