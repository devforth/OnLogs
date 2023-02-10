package containerdb

import "testing"

func Test_containStr(t *testing.T) {
	type args struct {
		a        string
		b        string
		caseSens bool
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"Not contain", args{a: "Amogus", b: "i", caseSens: false}, false},
		{"Contain without caseSens", args{a: "Amogus", b: "O", caseSens: false}, true},
		{"Contain, but caseSens", args{a: "Amogus", b: "O", caseSens: true}, false},
		{"Contain", args{a: "Amogus", b: "o", caseSens: true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containStr(tt.args.a, tt.args.b, tt.args.caseSens); got != tt.want {
				t.Errorf("containStr() = %v, want %v", got, tt.want)
			}
		})
	}
}
