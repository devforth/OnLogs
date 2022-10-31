package db

import (
	"fmt"
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

func GetLogs(container string, limit int, offset int) []string {
	var input struct {
		QueryString string `json:"query"`

		srchx.Query

		Join []struct {
			From string `json:"from"`

			*srchx.Join
		} `json:"join"`
	}
	input.Sort = append(input.Sort, "-timestamp")

	var q query.Query

	ndx, typ := container, "logs"
	index, err := vars.Store.GetIndex(ndx + "/" + typ)
	if err != nil {
		fmt.Println(err)
	}

	q = query.Query(bleve.NewMatchAllQuery())

	joins := []*srchx.Join{}
	for _, join := range input.Join {
		if join.From != "" {
			ndx, e := vars.Store.GetIndex(join.From)
			if e != nil {
				fmt.Println(err)
			}
			join.Join.Src = ndx
			joins = append(joins, join.Join)
		}
	}

	req := &srchx.Query{
		Query:  q,
		Offset: input.Offset,
		Size:   input.Size,
		Sort:   input.Sort,
		Join:   joins,
	}

	res, err := index.Search(req)

	if err != nil {
		fmt.Println(err)
	}

	to_return := []string{}
	for _, item := range res.Docs {
		to_return = append(to_return, item["message"].(string))
	}
	return to_return
}
