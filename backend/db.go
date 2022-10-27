package main

import (
	"strconv"
	"sync"
	"time"

	"github.com/blevesearch/bleve"
)

type Store struct {
	engine      string
	datapath    string
	indexes     map[string]*bleve.Index
	indexesLock sync.RWMutex
}

type LogItem struct {
	datetime string
	message  string
}

func storeItem(container string, item *LogItem) {
	datetime, _ := time.Parse(time.RFC3339Nano, item.datetime)
	timestamp := strconv.Itoa(int(datetime.UnixNano()))
	doc := map[string]interface{}{
		"id":      "",
		timestamp: item.message,
	}
	ndx, typ := container, "logs"
	index, _ := store.GetIndex(ndx + "/" + typ)
	doc, _ = index.Put(doc)
}
