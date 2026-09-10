package algo

import "testing"

func TestFirstChar(t *testing.T) {
	strings := []string{"leetcode", "", "aabb", "привет"}
	for _, s := range strings {
		r, number, ok := firstCharEasySolve(s)
		if !ok {
			t.Errorf("failed, string: %s, rune: %v, number: %d", s, r, number)
		}
		t.Logf("success, string: %s, rune: %v, number: %d", s, r, number)
	}
}
