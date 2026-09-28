STATUSES = ("pending", "done")


class TransitQueueStatus:
    IN_FLIGHT = "in_flight"


def test_upper_const():
    for status in STATUSES:
        assert status


def test_attr_const():
    for status in TransitQueueStatus.IN_FLIGHT:
        assert status
