package util

import (
	"testing"

	"github.com/devforth/OnLogs/app/vars"
)

func TestContains(t *testing.T) {
	type args struct {
		a    string
		list []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"Is contain 'a'", args{a: "a", list: []string{"a", "b", "c"}}, true},
		{"Is contain 'A'", args{a: "A", list: []string{"a", "b", "c"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Contains(tt.args.a, tt.args.list); got != tt.want {
				t.Errorf("Contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateInitUser(t *testing.T) {
	CreateInitUser()
	isExist, err := vars.UsersDB.Has([]byte("admin"), nil)
	if err != nil {
		t.Error(err.Error())
	}
	if !isExist {
		t.Error("User was not created!")
	}
}
