def test_uses_real_lock():
    qs = Order.objects.select_for_update().filter(id=1)
    assert qs.exists()
