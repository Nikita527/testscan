from df_app.service import mutate


def test_mutates_fixture(cart):
    mutate(cart)
    assert cart.touched
