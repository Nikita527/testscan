"""Own-code project module used by the only-happy-path fixtures."""


def process(value):
    if value < 0:
        raise ValueError("negative")
    return value


class Meta:
    def __init__(self, name):
        self.name = name
