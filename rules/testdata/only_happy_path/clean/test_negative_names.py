from happy_app.service import process


def test_ok_visible():
    process(0)
    assert result is True


def test_not_visible():
    process(0)
    assert result is False


def test_forbidden_in_environment():
    process(0)
    assert status == "forbidden"


def test_anonymous_is_denied():
    process(0)
    assert reason == "denied"
