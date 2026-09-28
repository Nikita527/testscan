from unittest.mock import patch


@patch("mod.compute", return_value=42)
def test_self_patch():
    assert mod.compute() == 42
