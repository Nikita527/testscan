def test_plan_free_limit():
    limit = limit_for("free")
    assert limit == 10


def test_plan_pro_limit():
    limit = limit_for("pro")
    assert limit == 100


def test_raises_match_differs():
    with pytest.raises(ValueError, match="empty name"):
        build("")


def test_raises_match_differs_for_blank():
    with pytest.raises(ValueError, match="blank name"):
        build(" ")
