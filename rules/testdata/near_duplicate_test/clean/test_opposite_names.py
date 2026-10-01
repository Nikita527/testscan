def test_job_demotes_on_timeout(queue):
    job = queue.run("slow")
    assert job.ok


def test_job_retries_on_timeout(queue):
    job = queue.run("flaky")
    assert job.ok


def test_event_passes_filter(flt):
    result = flt.apply("a")
    assert result is not None


def test_event_dropped_by_filter(flt):
    result = flt.apply("b")
    assert result is not None


def test_column_is_null(db):
    rows = db.query("a")
    assert rows


def test_column_is_not_null(db):
    rows = db.query("b")
    assert rows


def test_export_with_header(exporter):
    out = exporter.run("h")
    assert out


def test_export_without_header(exporter):
    out = exporter.run("n")
    assert out


def test_user_valid(checker):
    res = checker.run("x")
    assert res


def test_user_invalid(checker):
    res = checker.run("y")
    assert res
