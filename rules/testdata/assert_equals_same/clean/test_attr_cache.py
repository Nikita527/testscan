class Agent:
    @property
    def prop(self):
        return self


def test_cached_property_identity():
    a = Agent()
    assert a.prop is a.prop
