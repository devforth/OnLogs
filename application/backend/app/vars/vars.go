package vars

import (
	"github.com/gorilla/websocket"
	"github.com/syndtr/goleveldb/leveldb"
)

var (
	ActiveDBs                           = map[string]*leveldb.DB{}
	Stat_Containers_DBs                 = map[string]*leveldb.DB{}
	Stat_Hosts_DBs                      = map[string]*leveldb.DB{}
	Active_Daemon_Streams               = []string{}
	DockerContainers                    = []string{}
	Connections                         = map[string][]websocket.Conn{}
	Counters_For_Hosts_Last_30_Min      = map[string]map[string]int{}
	Counters_For_Containers_Last_30_Min = map[string]map[string]int{}
	FavsDB, _                           = leveldb.OpenFile("leveldb/favourites", nil)
	UsersDB, _                          = leveldb.OpenFile("leveldb/users", nil) // should i ever close it?
)

type UserData struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}