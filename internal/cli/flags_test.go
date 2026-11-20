package cli

import "testing"

func TestIsCLIArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"nil", nil, false},
		{"empty", []string{}, false},
		{"hands", []string{"--hands", "x"}, true},
		{"hands equals", []string{"--hands=x"}, true},
		{"short out", []string{"-o", "a.html"}, true},
		{"out equals", []string{"--out=a.html"}, true},
		{"check-ranges", []string{"--check-ranges"}, true},
		{"init-ranges equals", []string{"--init-ranges=d"}, true},
		{"help", []string{"-h"}, true},
		{"long help", []string{"--help"}, true},
		{"later known flag after unknown first", []string{"--foo", "--hands", "x"}, false},
		{"unknown", []string{"--foo"}, false},
		{"AppleLanguages", []string{"-AppleLanguages", "(en)"}, false},
		{"NS flag", []string{"-NSDocumentRevisionsDebugMode", "YES"}, false},
		{"psn", []string{"-psn_0_12345"}, false},
		{"file path", []string{"/Users/x/file.txt"}, false},
		{"flag prefix only", []string{"--hands-extra"}, false},
		{"single dash long", []string{"-hands"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCLIArgs(tt.args); got != tt.want {
				t.Errorf("IsCLIArgs(%q) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}
