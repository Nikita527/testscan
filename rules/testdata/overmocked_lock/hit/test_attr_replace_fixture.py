def test_race_attr_replace_fixture(sfu_mock):
    Order.objects.select_for_update = sfu_mock
    process_order(1)
    sfu_mock.assert_called_once()
