package dict

import (
	"strings"
	"testing"
)

func TestLoadReader(t *testing.T) {
	src := strings.NewReader("cat\nDOG\n\n  mouse  \n")
	d, err := LoadReader(src)
	if err != nil {
		t.Fatal(err)
	}
	if d.Size() != 3 {
		t.Errorf("size = %d, want 3", d.Size())
	}
	for _, w := range []string{"cat", "CAT", "Cat", "dog", "mouse"} {
		if !d.Contains(w) {
			t.Errorf("%q should be present", w)
		}
	}
	if d.Contains("horse") {
		t.Error("horse should not be present")
	}
}

func TestFromWords(t *testing.T) {
	d := FromWords([]string{"QI", "ZA", "AA"})
	for _, w := range []string{"qi", "za", "aa"} {
		if !d.Contains(w) {
			t.Errorf("%q should be present", w)
		}
	}
}
