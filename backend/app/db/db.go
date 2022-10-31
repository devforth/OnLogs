package db

import (
	"sync"
	"time"

	"github.com/blevesearch/bleve"
	"github.com/blevesearch/bleve/search/query"
	vars "github.com/devforth/OnLogs/app/vars"
	srchx "github.com/devforth/libsrchx"
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

type Query struct {
	Query  query.Query `json:"-"`
	Offset int         `json:"offset"`
	Size   int         `json:"size"`
	Sort   []string    `json:"sort"`
	Join   []*Join     `json:"join"`
}
type Join struct {
	Src   *bleve.Index `json:"-"`
	On    string       `json:"on"`
	As    string       `json:"as"`
	Where *Query       `json:"where"`
}

func StoreItem(container string, item *LogItem) {
	datetime, _ := time.Parse(time.RFC3339Nano, item.Datetime)
	doc := map[string]interface{}{
		"id":        "",
		"timestamp": int(datetime.UnixNano()),
		"message":   item.Message,
	}
	index, _ := vars.Store.GetIndex(container + "/logs")
	index.Put(doc)
}

func GetLogs(container string, message string, limit int, offset int) []string {
	var q query.Query
	if message == "" {
		q = query.Query(bleve.NewMatchAllQuery())
	} else {
		q = query.Query(bleve.NewMatchPhraseQuery(message))
	}

	index, _ := vars.Store.GetIndex(container + "/logs")
	res, _ := index.Search(
		&srchx.Query{
			Query:  q,
			Offset: offset,
			Size:   limit,
			Sort:   []string{"-timestamp"},
		},
	)

	to_return := []string{}
	for _, log_item := range res.Docs {
		to_return = append(to_return, log_item["message"].(string))
	}
	return to_return
}
