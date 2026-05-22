package imaging

import "testing"

func TestValidateGenerateSize(t *testing.T) {
	tests := []struct {
		name  string
		size  string
		valid bool
	}{
		{name: "default size", size: "1024x1024", valid: true},
		{name: "4k landscape", size: "3840x2160", valid: true},
		{name: "4k portrait", size: "2160x3840", valid: true},
		{name: "too large edge", size: "5120x2880", valid: false},
		{name: "too many pixels", size: "3840x3840", valid: false},
		{name: "ratio too wide", size: "3840x1024", valid: false},
		{name: "not multiple of 16", size: "1920x1080", valid: false},
		{name: "too few pixels", size: "512x512", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGenerateSize(tt.size)
			if tt.valid && err != nil {
				t.Fatalf("ValidateGenerateSize(%q) returned error: %v", tt.size, err)
			}
			if !tt.valid && err == nil {
				t.Fatalf("ValidateGenerateSize(%q) returned nil, want error", tt.size)
			}
		})
	}
}

func TestRequiresPaidGenerateAccount(t *testing.T) {
	tests := []struct {
		name string
		size string
		want bool
	}{
		{name: "empty size does not require paid", size: "", want: false},
		{name: "default size does not require paid", size: "1024x1024", want: false},
		{name: "free preset stays free", size: "1664x936", want: false},
		{name: "2k preset requires paid", size: "2560x1440", want: true},
		{name: "4k preset requires paid", size: "3840x2160", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RequiresPaidGenerateAccount(tt.size); got != tt.want {
				t.Fatalf("RequiresPaidGenerateAccount(%q) = %v, want %v", tt.size, got, tt.want)
			}
		})
	}
}
