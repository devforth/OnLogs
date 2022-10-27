package main

import srchx "github.com/devforth/libsrchx"

var (
	store, _ = srchx.NewStore("leveldb", ".")
)
