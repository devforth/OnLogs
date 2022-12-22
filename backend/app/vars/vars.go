package vars

import (
	"github.com/gorilla/websocket"
	"github.com/syndtr/goleveldb/leveldb"
)

var (
	UsersDB, _            = leveldb.OpenFile("/leveldb/users", nil) // should i ever close it?
	Connections           = map[string][]websocket.Conn{}
	All_Containers        = []string{}
	Active_Daemon_Streams = []string{}
	ActiveDBs             = map[string]*leveldb.DB{}
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

type UserLogin struct {
	Login string `json:"login"`
}
