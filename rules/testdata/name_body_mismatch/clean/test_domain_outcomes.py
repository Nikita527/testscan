def test_import_marks_row_failed():
    row = run_import(bad_payload)
    assert row.status == Status.FAILED


def test_sync_is_skipped_for_locked_items():
    outcome = sync(item)
    assert outcome is Outcome.SKIPPED


def test_transition_rejected():
    r = apply(order, "ship")
    assert r["status"] == "invalidTransition"


def test_validation_error_reported():
    result = validate(doc)
    assert result.outcome == "error"


def test_invalid_input_error_code():
    resp = handle(bad)
    assert resp.error_code == "E_BAD"


def test_invalid_rows_collected():
    report = check(rows)
    assert len(report.issues) > 0


def test_invalid_rows_truthy_errors():
    report = check(rows)
    assert report.errors


def test_invalid_issue_code():
    report = check(rows)
    assert report.issues[0].code == "dup"


def test_fails_with_message():
    job = run_job()
    assert job.error_message is not None


def test_fails_counts_failures():
    counters = batch(items)
    assert counters.failed == 2


def test_fails_counts_dict():
    counters = batch(items)
    assert counters["failed"] >= 1


def test_raises_via_try_else():
    try:
        explode()
    except ValueError:
        pass
    else:
        raise AssertionError("expected failure")


def test_rejects_blocked_user_without_calling_gateway(gateway):
    handle(blocked_user)
    gateway.send.assert_not_called()


def test_invalid_form_not_valid():
    form = Form(data={})
    assert not form.is_valid()


def test_invalid_result_not_ok():
    result = convert("zzz")
    assert not result.ok
