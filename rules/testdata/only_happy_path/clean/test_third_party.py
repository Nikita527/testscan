import requests
from rest_framework.test import APIClient


def test_a():
    assert APIClient().get("/a").status_code == 200


def test_b():
    assert APIClient().get("/b").status_code == 200


def test_c():
    assert requests.Session().get("/c") is not None


def test_d():
    assert APIClient().get("/d").status_code == 200
