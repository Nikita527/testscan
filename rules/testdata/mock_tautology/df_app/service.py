def process(x):
    if x < 0:
        raise ValueError(x)
    return x + 1


def activate(account_id):
    from df_app.models import Account

    Account.objects.filter(pk=account_id).update(active=True)


def place_order(user, repo):
    repo.save(user)


def mutate(cart):
    cart.touched = True
