package cmd

import (
	"os"
	"testing"
)

func TestColorEnabled(t *testing.T) {
	// A character device stands in for a terminal.
	device, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { device.Close() })
	pipeRead, pipeWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pipeRead.Close(); pipeWrite.Close() })
	file, err := os.Create(t.TempDir() + "/out.log")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { file.Close() })

	tests := []struct {
		name    string
		out     *os.File
		noColor string
		want    bool
	}{
		{"terminal", device, "", true},
		{"terminal with NO_COLOR", device, "1", false},
		{"pipe", pipeWrite, "", false},
		{"redirected to a file", file, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", tt.noColor)
			if got := colorEnabled(tt.out); got != tt.want {
				t.Errorf("colorEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}
