def test_procedural_boundary():
    do_side_effect()
    mock.assert_called_once()
