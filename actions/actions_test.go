package actions

import (
	"testing"

	"github.com/go-playground/assert/v2"
)

func TestValidatePrice(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want bool
	}{
		{
			name: "Success case. Correct price with digits after dot",
			arg:  "12332.02",
			want: true,
		},
		{
			name: "Success case. Correct price without digits after dot",
			arg:  "12323",
			want: true,
		},
		{
			name: "Fail case. Letter in price",
			arg:  "123q23",
			want: false,
		},
		{
			name: "Fail case. Letter in price with digits after dot",
			arg:  "123q23.02",
			want: false,
		},
		{
			name: "Fail case. Letter in price after dot",
			arg:  "123323.0t2",
			want: false,
		},
		{
			name: "Fail case. Two dots",
			arg:  "123.23.02",
			want: false,
		},
		{
			name: "Fail case. Two dots and letter in price",
			arg:  "123.23.0s2",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validatePrice(tt.arg)
			assert.Equal(t, tt.want, got)
		})
	}
}
