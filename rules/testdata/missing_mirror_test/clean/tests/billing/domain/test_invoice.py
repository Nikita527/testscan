from app.billing.domain.invoice import invoice


def test_invoice():
    assert invoice(1) == 1
