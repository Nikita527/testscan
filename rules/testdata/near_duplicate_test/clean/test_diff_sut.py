def test_resolve_product_field():
    assert resolve_product_field(row) == expected


def test_detect_product_filter_mode():
    assert detect_product_filter_mode(row) == expected
