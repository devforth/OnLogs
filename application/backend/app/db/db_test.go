package db

import "testing"

func TestIsTokenExists(t *testing.T) {
	type args struct {
		token string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"Bad token", args{token: "fasdfadsf"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTokenExists(tt.args.token); got != tt.want {
				t.Errorf("IsTokenExists() = %v, want %v", got, tt.want)
			}
		})
	}
}
