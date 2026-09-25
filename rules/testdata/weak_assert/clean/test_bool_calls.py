def test_serializer_valid():
    assert _S(data={"ids": [1, 2]}).is_valid()
    assert not _S(data={}).is_valid()


def test_errors_empty():
    errors = collect_errors()
    assert not errors


def test_predicate_call():
    assert is_failed_job_activity(job)
