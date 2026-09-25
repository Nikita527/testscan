from tests.blueprint.test_documents_api import _build_payload, _principal
from .test_warm_ast_contract import _build_ast

def test_ok():
    assert _build_payload() is not None
    assert _principal() is not None
    assert _build_ast() is not None
