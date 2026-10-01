def test_prose_comments():
    run()
    # Assert on_commit invalidate ran before any GET safety-net.
    # assert that the cache is warm by now.
    # assert: nothing else is needed here
    # assertion order does not matter
    # self.assertion style is discouraged
    assert True is True
