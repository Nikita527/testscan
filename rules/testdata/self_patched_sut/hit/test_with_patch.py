from unittest.mock import patch


def test_with_patch_body():
    with patch("mod.compute", return_value=42):
        assert mod.compute() == 42
