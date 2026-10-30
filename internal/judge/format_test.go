package judge

import "testing"

func TestFormatBinaryStorage(t *testing.T) {
	for _, tt := range []struct {
		value float64
		unit  string
		want  string
	}{
		{0, "B", "0 B"},
		{1, "B", "1 B"},
		{1023, "B", "1023 B"},
		{1024, "B", "1.0 KiB"},
		{1024 * 1024, "B", "1.0 MiB"},
		{62943 * 1024, "B", "61.5 MiB"},
		{62943, "KiB", "61.5 MiB"},
		{1024, "MiB", "1.0 GiB"},
		{1024, "GiB", "1.0 TiB"},
		{1024, "TiB", "1.0 PiB"},
		{1024, "PiB", "1.0 EiB"},
		{0.5, "MiB", "0.5 MiB"},
		{-62943, "KiB", "-61.5 MiB"},
		{0.02, "USD", "0.02 USD"},
	} {
		if got := Format(tt.value, tt.unit); got != tt.want {
			t.Errorf("Format(%v, %q) = %q, want %q", tt.value, tt.unit, got, tt.want)
		}
	}
}
