def test_permission_set_exact():
    perms = user_permissions(admin)
    assert perms == {"view_report", "edit_report", "delete_report", "view_user", "edit_user", "delete_user", "view_audit", "edit_audit", "view_billing", "edit_billing", "view_org", "edit_org", "delete_org"}


def test_error_codes_exact():
    codes = collect_codes(doc)
    assert codes == ["E_REQUIRED", "E_TOO_LONG", "E_BAD_FORMAT", "E_DUPLICATE", "E_UNKNOWN", "E_LOCKED", "E_EXPIRED", "E_EMPTY", "E_RANGE", "E_TYPE", "E_STATE", "E_ORDER", "E_LIMIT"]
