def _assert_pipeline_valid(result):
    assert result.ok
    assert result.errors == []


def test_via_private_helper():
    _assert_pipeline_valid(run())
