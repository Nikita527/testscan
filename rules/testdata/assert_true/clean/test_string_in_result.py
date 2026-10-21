def test_edge_checked():
    result = render()
    assert '"Agent1" -> "__end__";' in result
