class TestCheckTransitEnvironmentScope:
    def test_anonymous_is_denied(self):
        assert reason == "anonymous"


class TestRbacObjectByMethodMixin:
    def test_anonymous_is_denied(self):
        assert reason == "denied"
