"""Pure scalar mapper: no raise, returns str."""


def label_for(code: str) -> str:
    if code == "a":
        return "A"
    if code == "b":
        return "B"
    return code
