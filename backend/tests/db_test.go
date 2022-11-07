package db_test

import (
	"reflect"
	"testing"

	db "github.com/devforth/OnLogs/app/db"
)

func TestGetContainerLogs(t *testing.T) {
	type test struct {
		container string
		message   string
		limit     int
		offset    int
		want      []string
	}

	tests := []test{
		{container: "notest", message: "", limit: 0, offset: 0, want: []string{}},
		{container: "test", message: "", limit: 0, offset: 0, want: []string{"2022-10-31 13:15:40.974514944 +0000 UTC fgh 890",
			"2022-10-31 13:15:40.974512896 +0000 UTC def 678", "2022-10-31 13:15:40.974510592 +0000 UTC cde 456",
			"2022-10-31 13:15:40.974507264 +0000 UTC bcd 234", "2022-10-31 13:15:40.974485504 +0000 UTC abc 123"}},
		{container: "test", message: "", limit: 4, offset: 0, want: []string{"2022-10-31 13:15:40.974514944 +0000 UTC fgh 890",
			"2022-10-31 13:15:40.974512896 +0000 UTC def 678", "2022-10-31 13:15:40.974510592 +0000 UTC cde 456", "2022-10-31 13:15:40.974507264 +0000 UTC bcd 234"}},
		{container: "test", message: "", limit: 0, offset: 4, want: []string{"2022-10-31 13:15:40.974485504 +0000 UTC abc 123"}},
		{container: "test", message: "fgh", limit: 0, offset: 0, want: []string{"2022-10-31 13:15:40.974514944 +0000 UTC fgh 890"}},
		{container: "test", message: "deg 678", limit: 0, offset: 0, want: []string{}},
	}

	for _, tc := range tests {
		got := db.GetLogs(tc.container, tc.message, tc.limit, tc.offset)
		if !reflect.DeepEqual(tc.want, got) {
			t.Fatalf("expected: %v, got: %v", tc.want, got)
		}
	}
}
