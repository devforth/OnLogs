package db_test

import (
	"reflect"
	"testing"

	db "github.com/devforth/OnLogs/app/db"
)

func TestGetContainerLogs(t *testing.T) {
	type test struct {
		container string
		limit     int
		offset    int
		want      []string
	}

	tests := []test{
		{container: "notest", limit: 0, offset: 0, want: []string{}},
		{container: "test", limit: 0, offset: 0, want: []string{"f", "e", "d", "c", "b", "a"}},
		{container: "test", limit: 5, offset: 0, want: []string{"e", "d", "c", "b", "a"}},
		{container: "test", limit: 0, offset: 5, want: []string{"a"}},
	}

	for _, tc := range tests {
		got := db.GetLogs(tc.container, tc.limit, tc.offset)
		if !reflect.DeepEqual(tc.want, got) {
			t.Fatalf("expected: %v, got: %v", tc.want, got)
		}
	}
}
