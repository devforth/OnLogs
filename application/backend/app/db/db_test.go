package db

import (
	"testing"
)

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
		{"Valid token", args{token: CreateOnLogsToken()}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTokenExists(tt.args.token); got != tt.want {
				t.Errorf("IsTokenExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateOnLogsToken(t *testing.T) {
	token := CreateOnLogsToken()
	if !IsTokenExists(token) {
		t.Error("Invalid token")
	}
}
