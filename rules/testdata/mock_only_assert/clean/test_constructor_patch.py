from unittest.mock import patch


@patch("pkg.clients.ManagedIdentityCredential")
def test_constructor_patch():
    resolve()
    ManagedIdentityCredential.assert_called_once_with()
