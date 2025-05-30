package blinkdiskrunner

import (
	"os"
	"testing"
)

func TestBlinkDiskRunner(t *testing.T) {
	origEnv := os.Getenv("BLINKDISK_EXE")
	if origEnv == "" {
		t.Skip("Skipping blinkdisk runner test: 'BLINKDISK_EXE' is unset")
	}

	defer func() {
		envErr := os.Setenv("BLINKDISK_EXE", origEnv)
		if envErr != nil {
			t.Fatal("Unable to reset env BLINKDISK_EXE to original value")
		}
	}()

	for _, tt := range []struct {
		name            string
		exe             string
		args            []string
		expNewRunnerErr bool
		expRunErr       bool
	}{
		{
			name:            "empty exe",
			exe:             "",
			args:            nil,
			expNewRunnerErr: true,
			expRunErr:       false,
		},
		{
			name:            "invalid exe",
			exe:             "not-a-program",
			args:            []string{"some", "arguments"},
			expNewRunnerErr: false,
			expRunErr:       true,
		},
		{
			name:            "blinkdisk exe no args",
			exe:             origEnv,
			args:            []string{""},
			expNewRunnerErr: false,
			expRunErr:       true,
		},
		{
			name:            "blinkdisk exe help",
			exe:             origEnv,
			args:            []string{"--help"},
			expNewRunnerErr: false,
			expRunErr:       false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("BLINKDISK_EXE", tt.exe)

			runner, err := NewRunner("")
			if (err != nil) != tt.expNewRunnerErr {
				t.Fatalf("Expected NewRunner error: %v, got %v", tt.expNewRunnerErr, err)
			}

			if err != nil {
				return
			}

			defer runner.Cleanup()

			_, _, err = runner.Run(tt.args...)
			if (err != nil) != tt.expRunErr {
				t.Fatalf("Expected Run error: %v, got %v", tt.expRunErr, err)
			}
		})
	}
}
