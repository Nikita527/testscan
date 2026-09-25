def greet(name):
    if not name:
        raise ValueError("empty")
    return f"hi {name}"


def parse(data):
    try:
        return data["x"]
    except KeyError:
        return None
