package vars

import (
	"github.com/gorilla/websocket"
	"github.com/syndtr/goleveldb/leveldb"
)

var (
	UsersDB, _            = leveldb.OpenFile("leveldb/users", nil) // should i ever close it?
	Connections           = map[string][]websocket.Conn{}
	DockerContainers      = []string{}
	Active_Daemon_Streams = []string{}
	ActiveDBs             = map[string]*leveldb.DB{}
	FavsDB, _             = leveldb.OpenFile("leveldb/favourites", nil)
)

type UserData struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}
