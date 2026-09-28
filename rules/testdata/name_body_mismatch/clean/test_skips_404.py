def test_remove_member_skips_404():
    responses = {"/members/user-1/$ref": (404, None, "not found")}
    client.remove_group_member(group_id="group-1", user_id="user-1")
