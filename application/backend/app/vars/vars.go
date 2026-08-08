package vars

import (
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
)

var (
	ActiveDBs           = map[string]*leveldb.DB{}
	Stat_Containers_DBs = map[string]*leveldb.DB{}
	Stat_Hosts_DBs      = map[string]*leveldb.DB{}
	Statuses_DBs        = map[string]*leveldb.DB{}
	BrokenLogs_DBs      = map[string]*leveldb.DB{}
	ContainersMeta_DBs  = map[string]*leveldb.DB{}
	StreamState_DBs     = map[string]*leveldb.DB{}

	Active_Daemon_Streams = []string{}

	DockerContainers       = []string{}
	AgentsActiveContainers = map[string][]string{}

	ToDelete = map[string][]string{}

	// Reachable only through AddConnection/Broadcast/ConnectionCount so it
	// cannot be touched without the lock.
	connections = map[string][]*viewer{}

	Container_Stat_Counter = map[string]map[string]uint64{}

	Mutex            sync.Mutex
	DBMutex          sync.RWMutex
	connectionsMutex sync.RWMutex

	FavsDB, FavsDBErr         = leveldb.OpenFile("leveldb/favourites", nil)
	GroupsDB, GroupsDBErr     = leveldb.OpenFile("leveldb/groups", nil)
	StateDB, StateDBErr       = leveldb.OpenFile("leveldb/state", nil)
	UsersDB, UsersDBErr       = leveldb.OpenFile("leveldb/users", nil)
	TokensDB, TokensDBErr     = leveldb.OpenFile("leveldb/tokens", nil)
	SettingsDB, SettingsDBErr = leveldb.OpenFile("leveldb/usersSettings", nil)

	Year = strconv.Itoa(time.Now().UTC().Year())

	// Here rather than in routes so app/metrics need not import the HTTP layer.
	LoginFailures atomic.Uint64
	LoginBlocked  atomic.Uint64
)

const (
	StatisticsSaveInterval = 1 * time.Minute
)

type UserData struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}
