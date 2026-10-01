from df_app.models import Order
from df_app.service import process


def test_order_row_written(db):
    process(1)
    assert Order.objects.filter(pk=1).count() == 1
