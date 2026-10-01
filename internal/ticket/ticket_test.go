package ticket

import (
	"errors"
	"strings"
	"testing"
)

func TestInputNormalize(t *testing.T) {
	tests := []struct {
		name    string
		in      Input
		want    Input
		wantErr string
	}{
		{
			name: "applies defaults and trims",
			in:   Input{Title: "  Bug  ", Description: " desc ", Reporter: " ana "},
			want: Input{Title: "Bug", Description: "desc", Reporter: "ana", Status: StatusOpen, Priority: PriorityMedium},
		},
		{
			name: "keeps valid explicit values",
			in:   Input{Title: "Bug", Status: StatusClosed, Priority: PriorityHigh},
			want: Input{Title: "Bug", Status: StatusClosed, Priority: PriorityHigh},
		},
		{name: "missing title", in: Input{}, wantErr: "title is required"},
		{name: "whitespace title", in: Input{Title: "   "}, wantErr: "title is required"},
		{name: "title too long", in: Input{Title: strings.Repeat("a", 201)}, wantErr: "at most 200"},
		{name: "invalid status", in: Input{Title: "Bug", Status: "done"}, wantErr: "status must be"},
		{name: "invalid priority", in: Input{Title: "Bug", Priority: "urgent"}, wantErr: "priority must be"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := tc.in
			err := in.Normalize()
			if tc.wantErr != "" {
				var ve *ValidationError
				if !errors.As(err, &ve) {
					t.Fatalf("expected ValidationError, got %v", err)
				}
				if !strings.Contains(ve.Msg, tc.wantErr) {
					t.Fatalf("error %q does not contain %q", ve.Msg, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if in != tc.want {
				t.Fatalf("got %+v, want %+v", in, tc.want)
			}
		})
	}
}

func TestInputNormalizeTitleAtLimit(t *testing.T) {
	in := Input{Title: strings.Repeat("a", 200)}
	if err := in.Normalize(); err != nil {
		t.Fatalf("200-char title should be valid: %v", err)
	}
}
