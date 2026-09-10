package ws

import "testing"

func TestClientLogSeverity(t *testing.T) {
	tests := []struct {
		text string
		level string
		want string
	}{
		{
			text:  "changesError: Error\n    at CHistory._CheckCanNotAddChanges",
			level: "debug",
			want:  "debug",
		},
		{
			text:  "changesError: Error: Uncaught TypeError: this.NewClass.Write_ToBinary2 is not a function",
			level: "error",
			want:  "error",
		},
		{
			text:  "something else",
			level: "error",
			want:  "warn",
		},
	}
	for _, tc := range tests {
		if got := clientLogSeverity(tc.text, tc.level); got != tc.want {
			t.Fatalf("clientLogSeverity(%q, %q) = %q, want %q", tc.text, tc.level, got, tc.want)
		}
	}
}
