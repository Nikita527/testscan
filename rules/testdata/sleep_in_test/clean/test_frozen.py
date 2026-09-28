from freezegun import freeze_time
from datetime import datetime


@freeze_time("2024-01-01")
def test_frozen_now():
    assert datetime.now().year == 2024
