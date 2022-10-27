package db

import (
	"strconv"
	"sync"
	"time"

	"github.com/blevesearch/bleve"
	vars "github.com/devforth/OnLogs/app/vars"
)

type Store struct {
	engine      string
	datapath    string
	indexes     map[string]*bleve.Index
	indexesLock sync.RWMutex
}

type LogItem struct {
	Datetime string
	Message  string
}

func StoreItem(container string, item *LogItem) {
	datetime, _ := time.Parse(time.RFC3339Nano, item.Datetime)
	timestamp := strconv.Itoa(int(datetime.UnixNano()))
	doc := map[string]interface{}{
		"id":      "",
		timestamp: item.Message,
	}
	ndx, typ := container, "logs"
	index, _ := vars.Store.GetIndex(ndx + "/" + typ)
	doc, _ = index.Put(doc)
}
