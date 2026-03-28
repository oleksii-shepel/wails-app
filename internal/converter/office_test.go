package converter

import "testing"

func TestImagemagickFormatCanRead(t *testing.T) {
	tests := []struct {
		name   string
		output string
		format string
		want   bool
	}{
		{
			name:   "readable format",
			output: "     PNG* PNG       rw-   Portable Network Graphics\n",
			format: "png",
			want:   true,
		},
		{
			name:   "format present without read delegate",
			output: "     DJVU* DJVU      ---   Deja vu\n",
			format: "djvu",
			want:   false,
		},
		{
			name:   "format not listed",
			output: "     PNG* PNG       rw-   Portable Network Graphics\n",
			format: "djvu",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := imagemagickFormatCanRead(tt.output, tt.format)
			if got != tt.want {
				t.Fatalf("imagemagickFormatCanRead(%q) = %v, want %v", tt.format, got, tt.want)
			}
		})
	}
}
