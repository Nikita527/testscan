from df_app.service import activate


def test_activate_persists(account):
    activate(1)
    account.refresh_from_db()
    assert account.active
