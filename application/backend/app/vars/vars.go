package vars

import (
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/syndtr/goleveldb/leveldb"
)

var (
	ActiveDBs           = map[string]*leveldb.DB{}
	Stat_Containers_DBs = map[string]*leveldb.DB{}
	Stat_Hosts_DBs      = map[string]*leveldb.DB{}
	Statuses_DBs        = map[string]*leveldb.DB{}
	BrokenLogs_DBs      = map[string]*leveldb.DB{}
	ContainersMeta_DBs  = map[string]*leveldb.DB{}

	Active_Daemon_Streams = []string{}

	DockerContainers       = []string{}
	AgentsActiveContainers = map[string][]string{}

	ToDelete    = map[string][]string{}
	Connections = map[string][]websocket.Conn{}

	Counters_For_Hosts_Last_30_Min = map[string]map[string]uint64{}
	Container_Stat_Counter         = map[string]map[string]uint64{}

	Mutex   sync.Mutex
	DBMutex sync.RWMutex

	FavsDB, FavsDBErr         = leveldb.OpenFile("leveldb/favourites", nil)
	StateDB, StateDBErr       = leveldb.OpenFile("leveldb/state", nil)
	UsersDB, UsersDBErr       = leveldb.OpenFile("leveldb/users", nil)
	TokensDB, TokensDBErr     = leveldb.OpenFile("leveldb/tokens", nil)
	SettingsDB, SettingsDBErr = leveldb.OpenFile("leveldb/usersSettings", nil)

	Year = strconv.Itoa(time.Now().UTC().Year())
)

const (
	StatisticsSaveInterval = 1 * time.Minute
)

type UserData struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}
