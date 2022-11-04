package vars

import srchx "github.com/devforth/libsrchx"

var (
	Store, _ = srchx.NewStore("leveldb", ".")
)
