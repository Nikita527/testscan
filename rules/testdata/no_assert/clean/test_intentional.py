from df_app.service import process


def test_process_smoke():
    process(1)


def test_process_does_not_propagate_errors():
    process(2)


def test_plain_name_with_comment():
    # must not raise for empty input
    process(0)
