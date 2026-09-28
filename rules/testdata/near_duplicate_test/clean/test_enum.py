def test_managed_identity():
    mode = resolve_auth()
    assert mode == "MANAGED_IDENTITY"


def test_default_chain():
    mode = resolve_auth()
    assert mode == "DEFAULT_CHAIN"
