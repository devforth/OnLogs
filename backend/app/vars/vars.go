package vars

import (
	srchx "github.com/devforth/libsrchx"
	"github.com/gorilla/websocket"
	"github.com/nsqio/go-diskqueue"
	"github.com/syndtr/goleveldb/leveldb"
)

var (
	Store, _              = srchx.NewStore("srchxdb", ".")
	UsersDB, _            = leveldb.OpenFile("onlogsdb", nil) // should i ever close it?
	Connections           = map[string][]websocket.Conn{}
	Active_DQ             = map[string]diskqueue.Interface{}
	All_Containers        = []string{}
	Active_Daemon_Streams = []string{}
)

type Container struct {
	Id      string   `json:"Id"`
	Names   []string `json:"Names"`
	Image   string   `json:"Image"`
	ImageID string   `json:"ImageID"`
	Data    []struct {
		ID        int    `json:"id"`
		Email     string `json:"email"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Avatar    string `json:"avatar"`
	} `json:"data"`
	Support struct {
		URL  string `json:"url"`
		Text string `json:"text"`
	} `json:"support"`
}

type HostsList struct {
	Host     string   `json:"host"`
	Services []string `json:"services"`
}

type UserData struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}
