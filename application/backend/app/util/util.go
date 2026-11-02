package util

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/devforth/OnLogs/app/userdb"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/golang-jwt/jwt/v5"
	"github.com/syndtr/goleveldb/leveldb"
	leveldbErrors "github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"golang.org/x/sys/unix"
)

func replaceVarForAllFilesInDir(dirName string, dir_files []fs.DirEntry) {
	for _, dir_file := range dir_files {
		if strings.HasSuffix(dir_file.Name(), ".js") || strings.HasSuffix(dir_file.Name(), ".css") || strings.HasSuffix(dir_file.Name(), ".html") {
			input, err := os.ReadFile("dist/" + dirName + "/" + dir_file.Name())
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			output := bytes.Replace(input, []byte("/ONLOGS_PREFIX_ENV_VARIABLE_THAT_SHOULD_BE_REPLACED_ON_BACKEND_INITIALIZATION"), []byte(os.Getenv("ONLOGS_PATH_PREFIX")), -1)
			if err = os.WriteFile("dist/"+dirName+"/"+dir_file.Name(), output, 0666); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}
	}
}

func CreateInitUser() error {
	admin_username := os.Getenv("ADMIN_USERNAME")
	if admin_username == "" {
		admin_username = "admin"
		os.Setenv("ADMIN_USERNAME", admin_username)
	}

	admin_password := os.Getenv("ADMIN_PASSWORD")
	if admin_password == "" {
		return errors.New("ADMIN_PASSWORD is empty; refusing to create an administrator that would accept an empty password")
	}

	unchanged := userdb.CheckUserPassword(admin_username, admin_password)
	if err := vars.UsersDB.Put([]byte(admin_username), []byte(userdb.HashPassword(admin_password)), nil); err != nil {
		return err
	}
	if unchanged {
		return nil
	}
	return userdb.RevokeSessions(admin_username)
}

func ReplacePrefixVariableForFrontend() {
	files, err := os.ReadDir("dist")
	if err != nil {
		fmt.Println("INFO: unable to find 'dist' folder")
		return
	}
	for _, file := range files {
		if file.IsDir() {
			dir_files, _ := os.ReadDir("dist/" + file.Name())
			replaceVarForAllFilesInDir(file.Name(), dir_files)
		}
	}
	replaceVarForAllFilesInDir("", files)
}

func CreateJWT(login string) string {
	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	claims["exp"] = time.Now().AddDate(0, 0, 2).Unix()
	claims["user"] = login
	claims["gen"], _ = userdb.SessionGeneration(login)
	// Two logins in the same second would otherwise share a cookie, and
	// logging out of one would end both.
	claims["jti"] = rand.Text()
	tokenString, _ := token.SignedString([]byte(os.Getenv("JWT_SECRET")))

	return tokenString
}

// Every path component must be a single, literal name; anything else escapes
// leveldb/hosts.
func IsSafeName(s string) bool {
	if s == "" || s == "." || s == ".." {
		return false
	}
	// filepath.Base("/") is "/", so the Base comparison alone lets it through.
	if strings.ContainsAny(s, `/\`) || strings.ContainsRune(s, 0) {
		return false
	}
	return s == filepath.Base(s)
}

// Level 1 at 10 MiB, as in LevelDB, not 100: each retention round rewrites it.
var DBOptions = &opt.Options{CompactionTotalSizeMultiplierPerLevel: []float64{1, 1}}

func GetDB(host string, container string, dbType string) *leveldb.DB {
	if !IsSafeName(host) || !IsSafeName(container) || !IsSafeName(dbType) {
		return nil
	}

	vars.DBMutex.RLock()
	db := getExistingDB(host, container, dbType)
	vars.DBMutex.RUnlock()
	if db != nil {
		return db
	}

	vars.DBMutex.Lock()
	defer vars.DBMutex.Unlock()

	db = getExistingDB(host, container, dbType)
	if db != nil {
		return db
	}

	path := fmt.Sprintf("leveldb/hosts/%s/containers/%s/%s", host, container, dbType)
	db, err := leveldb.OpenFile(path, DBOptions)

	// Recovery rewrites the whole DB: only for corruption, not for a full disk.
	for tries := 0; err != nil && tries < 10; tries++ {
		time.Sleep(10 * time.Millisecond)
		if leveldbErrors.IsCorrupted(err) {
			db, err = leveldb.RecoverFile(path, DBOptions)
		} else {
			db, err = leveldb.OpenFile(path, DBOptions)
		}
	}

	if err != nil {
		fmt.Printf("ERROR: unable to open db for %s/%s/%s: %v\n", host, container, dbType, err)
		return nil
	}

	// Assigning into a nil map panics, unlike reading or deleting.
	if cache := dbCaches[dbType]; cache != nil {
		cache[host+"/"+container] = db
	}
	return db
}

// GetDBIfExists is the read path: it never creates. GetDB opens-or-creates and
// caches forever.
func GetDBIfExists(host string, container string, dbType string) *leveldb.DB {
	if !IsSafeName(host) || !IsSafeName(container) {
		return nil
	}
	if info, err := os.Stat("leveldb/hosts/" + host + "/containers/" + container); err != nil || !info.IsDir() {
		return nil
	}
	return GetDB(host, container, dbType)
}

func GetContainersMetaDB(host string) *leveldb.DB {
	if !IsSafeName(host) {
		return nil
	}

	vars.DBMutex.RLock()
	db := vars.ContainersMeta_DBs[host]
	vars.DBMutex.RUnlock()
	if db != nil {
		return db
	}

	vars.DBMutex.Lock()
	defer vars.DBMutex.Unlock()

	if db = vars.ContainersMeta_DBs[host]; db != nil {
		return db
	}

	db, err := leveldb.OpenFile("leveldb/hosts/"+host+"/containersMeta", nil)
	if err != nil {
		fmt.Printf("ERROR: unable to open containersMeta for %s: %v\n", host, err)
		return nil
	}
	vars.ContainersMeta_DBs[host] = db
	return db
}

func ResetDB(host string, container string, dbType string) {
	if !IsSafeName(host) || !IsSafeName(container) || !IsSafeName(dbType) {
		return
	}

	vars.DBMutex.Lock()
	defer vars.DBMutex.Unlock()

	if db := getExistingDB(host, container, dbType); db != nil {
		db.Close()
	}
	delete(dbCaches[dbType], host+"/"+container)
	delete(lastReopen, dbType+":"+host+"/"+container)
}

var FreeDiskBytes = func() (uint64, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs("leveldb", &stat); err != nil {
		return 0, err
	}
	return stat.Bavail * uint64(stat.Bsize), nil
}

var lastReopen = map[string]time.Time{}

// ReopenDB clears goleveldb's sticky write error, at most every 10 s.
func ReopenDB(host string, container string, dbType string, failed *leveldb.DB) *leveldb.DB {
	if free, err := FreeDiskBytes(); err != nil || free >= uint64(2*opt.DefaultWriteBuffer) {
		key := dbType + ":" + host + "/" + container
		vars.DBMutex.Lock()
		if db := getExistingDB(host, container, dbType); db != nil && db == failed && time.Since(lastReopen[key]) >= 10*time.Second {
			db.Close()
			delete(dbCaches[dbType], host+"/"+container)
			lastReopen[key] = time.Now()
		}
		vars.DBMutex.Unlock()
	}
	return GetDB(host, container, dbType)
}

// CloseDBs closes every database for good: only on the way out.
func CloseDBs() {
	vars.DBMutex.Lock()
	for _, cache := range dbCaches {
		for _, db := range cache {
			db.Close()
		}
	}
	for _, db := range vars.ContainersMeta_DBs {
		db.Close()
	}
	for _, db := range []*leveldb.DB{vars.AliasDB, vars.FavsDB, vars.GroupsDB, vars.UsersDB, vars.TokensDB, vars.SettingsDB, vars.SessionsDB, vars.EndedSessionsDB} {
		if db != nil {
			db.Close()
		}
	}
}

// ContainersMeta_DBs is absent: GetContainersMetaDB keys it by bare host.
var dbCaches = map[string]map[string]*leveldb.DB{
	"logs":        vars.ActiveDBs,
	"statistics":  vars.Stat_Containers_DBs,
	"statuses":    vars.Statuses_DBs,
	"brokenlogs":  vars.BrokenLogs_DBs,
	"streamstate": vars.StreamState_DBs,
}

func getExistingDB(host, container, dbType string) *leveldb.DB {
	return dbCaches[dbType][host+"/"+container]
}

// AGENT is documented as a boolean, so "false" must mean off.
func IsAgentMode() bool {
	enabled, err := strconv.ParseBool(os.Getenv("AGENT"))
	return err == nil && enabled
}

func parseHostname(raw string, readErr error) string {
	if readErr != nil {
		fallback, _ := os.Hostname()
		raw = fallback
	}

	host := strings.TrimFunc(raw, func(r rune) bool { return r < 32 || r > 126 })
	if host == "" {
		if fallback, err := os.Hostname(); err == nil {
			host = strings.TrimSpace(fallback)
		}
	}
	if host == "" {
		host = "localhost"
	}
	return host
}

var hostID atomic.Pointer[string]

func SetHost(id string) { hostID.Store(&id) }

func GetHost() string {
	if id := hostID.Load(); id != nil {
		return *id
	}
	hostname, err := os.ReadFile("/etc/hostname")
	return parseHostname(string(hostname), err)
}

func RemoveHostIfEmpty(host string) (bool, error) {
	if !IsSafeName(host) {
		return false, nil
	}
	dir := "leveldb/hosts/" + host

	vars.DBMutex.Lock()
	defer vars.DBMutex.Unlock()
	containers, err := os.ReadDir(dir + "/containers")
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	for _, c := range containers {
		db := getExistingDB(host, c.Name(), "logs")
		if db == nil {
			return false, nil
		}
		iter := db.NewIterator(nil, nil)
		rows, err := iter.First(), iter.Error()
		iter.Release()
		if rows || err != nil {
			return false, err
		}
	}
	for dbType, cache := range dbCaches {
		for _, c := range containers {
			key := host + "/" + c.Name()
			if db := cache[key]; db != nil {
				db.Close()
			}
			delete(cache, key)
			delete(lastReopen, dbType+":"+key)
		}
	}
	if db := vars.ContainersMeta_DBs[host]; db != nil {
		db.Close()
		delete(vars.ContainersMeta_DBs, host)
	}
	return true, os.RemoveAll(dir)
}

// LogsSizes reports on-disk bytes per host/container plus the total. It counts
// whole directories, so it can exceed MAX_LOGS_SIZE by data retention cannot
// delete (stream cursors, agent buffers). One filepath.Walk per container: never
// call it on a request that runs often.
func LogsSizes() (map[string]int64, int64) {
	perContainer := map[string]int64{}
	var total int64

	hosts, _ := os.ReadDir("leveldb/hosts/")
	for _, host := range hosts {
		containers, _ := os.ReadDir("leveldb/hosts/" + host.Name() + "/containers")
		for _, container := range containers {
			bytes := int64(GetDirSize(host.Name(), container.Name()) * 1024 * 1024)
			perContainer[host.Name()+"/"+container.Name()] = bytes
			total += bytes
		}
	}
	return perContainer, total
}

func GetDirSize(host string, container string) float64 {
	if !IsSafeName(host) || !IsSafeName(container) {
		return 0
	}

	var size int64

	path := "leveldb/hosts/" + host + "/containers/" + container
	_, pathErr := os.Stat(path)
	if os.IsNotExist(pathErr) {
		return 0
	}

	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if info != nil {
			if !info.IsDir() {
				size += info.Size()
			}
			return err
		}
		return nil
	})
	return float64(size) / (1024.0 * 1024.0)
}

// prunableDBs are the sub-databases retention actually deletes from. Measuring
// anything else makes the quota unreachable: retention would strip these to
// nothing trying to offset data it cannot touch.
var prunableDBs = []string{"logs", "statuses", "statistics"}

func GetPrunableSize(host string, container string) float64 {
	if !IsSafeName(host) || !IsSafeName(container) {
		return 0
	}

	var size int64
	for _, dbType := range prunableDBs {
		path := "leveldb/hosts/" + host + "/containers/" + container + "/" + dbType
		filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info != nil && !info.IsDir() {
				size += info.Size()
			}
			return nil
		})
	}
	return float64(size) / (1024.0 * 1024.0)
}

func PrunableDBs() []string {
	return append([]string{}, prunableDBs...)
}

func GetUserFromJWT(req http.Request) (string, error) {
	user, _, _, err := GetSessionFromJWT(req)
	return user, err
}

// The session key names this one cookie; exp is when it lapses on its own.
func GetSessionFromJWT(req http.Request) (user string, session string, exp int64, err error) {
	unauthorized := errors.New("401 - Unauthorized!")
	c, _ := req.Cookie("onlogs-cookie")
	if c == nil {
		return "", "", 0, unauthorized
	}

	// An empty key verifies every HMAC token.
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", "", 0, unauthorized
	}

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(c.Value, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())

	if err != nil {
		return "", "", 0, err
	}

	expClaim, ok := claims["exp"].(float64)
	if !ok || int64(expClaim) < time.Now().Unix() {
		return "", "", 0, errors.New("Token is expired")
	}

	user, ok = claims["user"].(string)
	if !ok || user == "" {
		return "", "", 0, unauthorized
	}

	// A valid signature is not enough: deleting an account must revoke it.
	if exists, err := vars.UsersDB.Has([]byte(user), nil); err != nil || !exists {
		return "", "", 0, unauthorized
	}
	// Sessions issued before generations existed carry none and read as 0.
	gen, _ := claims["gen"].(float64)
	if current, err := userdb.SessionGeneration(user); err != nil || gen != float64(current) {
		return "", "", 0, unauthorized
	}
	// The decoded signature, not the cookie text: base64 lets one signature be
	// spelled several ways. It also names cookies issued without a jti.
	sum := sha256.Sum256(token.Signature)
	session = hex.EncodeToString(sum[:])
	if ended, err := userdb.IsSessionEnded(session); err != nil || ended {
		return "", "", 0, unauthorized
	}
	return user, session, int64(expClaim), nil
}

// Mints agent ingestion tokens, so it must be cryptographically random.
func GenerateJWTSecret() string {
	return rand.Text()
}

func GetDockerContainerID(host string, container string) string {
	if !IsSafeName(host) {
		return ""
	}

	_, err := os.ReadDir("leveldb/hosts/" + host)
	if err != nil {
		return ""
	}

	containersMetaDB := GetContainersMetaDB(host)
	if containersMetaDB == nil {
		return ""
	}

	iter := containersMetaDB.NewIterator(nil, nil)
	defer iter.Release()

	iter.Last()
	for iter.Key() != nil {
		if string(iter.Key()) == container {
			return string(iter.Value())
		}
		iter.Prev()
	}

	return ""
}

var DockerContainersDir = "/var/lib/docker/containers/"

// Docker stops following a log file cleared under a running container.
var errContainerRunning = errors.New("Stop the container first: Docker sends no new lines once a running container's log is cleared")

func DeleteDockerLogs(host string, container string) error {
	if host != GetHost() {
		if slices.Contains(vars.AgentContainers(host), container) {
			return errContainerRunning
		}
		vars.QueueDelete(host, container)
		return nil
	}
	if slices.Contains(vars.DockerContainerList(), container) {
		return errContainerRunning
	}

	containerID := GetDockerContainerID(host, container)
	if containerID == "" {
		return errors.New("no docker container id for " + container)
	}
	return os.Truncate(DockerContainersDir+containerID+"/"+containerID+"-json.log", 0)
}

func GetStorageData() map[string]float64 {
	var stat unix.Statfs_t
	wd, _ := os.Getwd()
	unix.Statfs(wd, &stat)

	total_space_GB := float64(stat.Blocks*uint64(stat.Bsize)) / (1000 * 1000 * 1000)
	free_space_GB := float64(stat.Bfree*uint64(stat.Bsize)) / (1000 * 1000 * 1000)
	return map[string]float64{
		"total_space_GB":     total_space_GB,
		"free_space_GB":      free_space_GB,
		"free_space_percent": (free_space_GB / total_space_GB) * 100,
	}
}

var units = []struct {
	Suffix     string
	Multiplier int64
}{
	{"TB", 1024 * 1024 * 1024 * 1024},
	{"T", 1024 * 1024 * 1024 * 1024},
	{"GB", 1024 * 1024 * 1024},
	{"G", 1024 * 1024 * 1024},
	{"MB", 1024 * 1024},
	{"M", 1024 * 1024},
	{"KB", 1024},
	{"K", 1024},
	{"B", 1},
}

func ParseHumanReadableSize(sizeStr string) (int64, error) {
	sizeStr = strings.TrimSpace(strings.ToUpper(sizeStr))
	for _, unit := range units {
		if strings.HasSuffix(sizeStr, unit.Suffix) {
			numStr := strings.TrimSuffix(sizeStr, unit.Suffix)
			num, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid number in size: %s", numStr)
			}
			return int64(num * float64(unit.Multiplier)), nil
		}
	}
	return 0, fmt.Errorf("unknown size unit in: %s", sizeStr)
}
