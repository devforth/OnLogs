package containerdb

import (
	"testing"
)

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

// func TestPutLogMessage(t *testing.T) {
// 	cont := "testCont"
// 	host := "testHost"
// 	vars.Counters_For_Hosts_Last_30_Min[host+"/"+cont] = map[string]int{"error": 0, "debug": 0, "info": 0, "warn": 0, "other": 0}
// 	db, _ := leveldb.OpenFile("leveldb/hosts"+host+"/container/"+cont, nil)
// 	defer db.Close()

// 	PutLogMessage(db, host, cont, []string{"2023-02-10T12:56:09.230421754Z", "vokAU6OdSulJGynsz wBaKssXuAPGk6ZFiQxq4sQHe7B9Q9RbTAy\r\n"})
// 	has, _ := db.Has([]byte("2023-02-10T12:56:09.230421754Z"), nil)
// 	if !has {
// 		t.Error("a")
// 	}
// }
