package web

import "testing"

func TestValidReadingEvent(t *testing.T) {
	tests := []struct {
		name string
		req  readingEventRequest
		want bool
	}{
		{
			name: "valid",
			req:  readingEventRequest{ImpressionID: 1, MaxScrollPct: 80, DwellSeconds: 30},
			want: true,
		},
		{
			name: "missing impression",
			req:  readingEventRequest{MaxScrollPct: 80, DwellSeconds: 30},
			want: false,
		},
		{
			name: "bad scroll",
			req:  readingEventRequest{ImpressionID: 1, MaxScrollPct: 101, DwellSeconds: 30},
			want: false,
		},
		{
			name: "bad dwell",
			req:  readingEventRequest{ImpressionID: 1, MaxScrollPct: 80, DwellSeconds: 90000},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validReadingEvent(tt.req); got != tt.want {
				t.Fatalf("validReadingEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}
