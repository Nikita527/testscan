package rules

import "testing"

func TestCallParenSpanEnd_IgnoresCommentParens(t *testing.T) {
	lines := []string{
		`with patch(  # TODO (revisit later`,
		`    "app.services.notify"`,
		`) as mock_notify:`,
		`    Order.objects.select_for_update().filter(id=1)`,
		`    mock_notify.assert_called_once()`,
	}
	end := callParenSpanEnd(lines, 0, len(lines))
	if end != 3 {
		t.Fatalf("span end want 3 (closing paren line), got %d", end)
	}
}
