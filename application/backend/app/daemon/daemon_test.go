package daemon

import (
	"reflect"
	"testing"
)

func Test_validateMessage(t *testing.T) {
	type args struct {
		message string
	}
	tests := []struct {
		name  string
		args  args
		want  string
		want1 bool
	}{
		{"Bad message", args{message: string([]byte{10})}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := validateMessage(tt.args.message)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("validateMessage() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("validateMessage() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
