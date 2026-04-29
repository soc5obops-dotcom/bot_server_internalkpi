package render

import "testing"

func TestRenderWidthsIncludesConfiguredWidthAndFallbacks(t *testing.T) {
	widths := renderWidths(2400)

	if len(widths) < 2 {
		t.Fatalf("expected fallback widths, got %v", widths)
	}
	if widths[0] != 2400 {
		t.Fatalf("first width = %d, want configured width 2400", widths[0])
	}
	if widths[len(widths)-1] != minRenderWidth {
		t.Fatalf("last width = %d, want minimum width %d", widths[len(widths)-1], minRenderWidth)
	}
	for i := 1; i < len(widths); i++ {
		if widths[i] >= widths[i-1] {
			t.Fatalf("widths should decrease: %v", widths)
		}
		if widths[i] < minRenderWidth {
			t.Fatalf("widths should not go below %d: %v", minRenderWidth, widths)
		}
	}
}

func TestRenderWidthsKeepsSmallConfiguredWidth(t *testing.T) {
	widths := renderWidths(1000)

	if len(widths) != 1 || widths[0] != 1000 {
		t.Fatalf("widths = %v, want only configured width", widths)
	}
}
