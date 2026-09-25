def test_ok_visible():
    assert result is True


def test_not_visible():
    assert result is False


def test_forbidden_in_environment():
    assert status == "forbidden"


def test_anonymous_is_denied():
    assert reason == "denied"
