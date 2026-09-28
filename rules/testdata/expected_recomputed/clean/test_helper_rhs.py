def test_helper_call_on_rhs_ok():
    assert "/" not in rules_file_for_mapping("sales")
    assert rules_file_for_mapping("sales") == "sales.json"
