class RunUsage:
    def __init__(self, input_tokens=0):
        self.input_tokens = input_tokens

    def __eq__(self, other):
        return self.input_tokens == other.input_tokens


def test_usage_equals_constructor():
    got = measure()
    assert got == RunUsage(input_tokens=7)
