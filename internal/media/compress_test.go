package media

import "testing"

func TestShouldReplaceWithCompressed(t *testing.T) {
	tests := []struct {
		name       string
		original   int64
		compressed int64
		want       bool
	}{
		{name: "smaller enough", original: 1000, compressed: 900, want: true},
		{name: "barely smaller", original: 1000, compressed: 980, want: false},
		{name: "larger", original: 1000, compressed: 1100, want: false},
		{name: "empty", original: 1000, compressed: 0, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldReplaceWithCompressed(tt.original, tt.compressed); got != tt.want {
				t.Fatalf("shouldReplaceWithCompressed(%d, %d) = %v, want %v", tt.original, tt.compressed, got, tt.want)
			}
		})
	}
}
