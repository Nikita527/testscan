def test_does_not_fail_on_empty_input():
    assert run([]) == []


def test_never_raises_for_unknown_key():
    assert lookup("zz") == 0


def test_fails_open_when_cache_down():
    assert get(1) == 1


def test_fallback_when_parser_fails():
    assert parse("x") == "x"


def test_noop_when_nothing_invalid():
    assert clean([]) == []


def test_survives_when_worker_fails():
    assert pool.alive()


def test_completes_when_step_two_fails():
    assert pipeline.done()


def test_bulk_reject_marks_all():
    assert bulk_reject(ids) == 3


def test_reject_persists_reason():
    item = reject(order)
    assert item.reason_text == "no"


def test_bulk_reopen_restores_a_rejected_package():
    pkg = reopen(rejected_pkg)
    assert pkg.state == "open"
