package util

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/devforth/OnLogs/app/userdb"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/golang-jwt/jwt/v5"
	"github.com/syndtr/goleveldb/leveldb"
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

func Contains(a string, list []string) bool {
	for _, b := range list {
		if strings.Compare(b, a) == 0 {
			return true
		}
	}
	return false
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

	return vars.UsersDB.Put([]byte(admin_username), []byte(userdb.HashPassword(admin_password)), nil)
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
	claims["authorized"] = true
	claims["user"] = login
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
	db, err := leveldb.OpenFile(path, nil)

	if err != nil {
		for tries := 0; tries < 10; tries++ {
			db, err = leveldb.RecoverFile(path, nil)
			if err == nil {
				break
			}
			fmt.Printf("Attempt %d to recover DB %s failed: %v\n", tries+1, path, err)
			time.Sleep(10 * time.Millisecond)
		}
	}

	if err != nil {
		fmt.Printf("ERROR: unable to open db for %s/%s/%s: %v\n", host, container, dbType, err)
		return nil
	}

	switch dbType {
	case "logs":
		vars.ActiveDBs[host+"/"+container] = db
	case "statistics":
		vars.Stat_Containers_DBs[host+"/"+container] = db
	case "hosts_statistics":
		vars.Stat_Hosts_DBs[host] = db
	case "statuses":
		vars.Statuses_DBs[host+"/"+container] = db
	case "brokenlogs":
		vars.BrokenLogs_DBs[host+"/"+container] = db
	case "containersmeta":
		vars.ContainersMeta_DBs[host+"/"+container] = db
	case "streamstate":
		vars.StreamState_DBs[host+"/"+container] = db
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

	switch dbType {
	case "logs":
		delete(vars.ActiveDBs, host+"/"+container)
	case "statistics":
		delete(vars.Stat_Containers_DBs, host+"/"+container)
	case "hosts_statistics":
		delete(vars.Stat_Hosts_DBs, host)
	case "statuses":
		delete(vars.Statuses_DBs, host+"/"+container)
	case "brokenlogs":
		delete(vars.BrokenLogs_DBs, host+"/"+container)
	case "containersmeta":
		delete(vars.ContainersMeta_DBs, host+"/"+container)
	case "streamstate":
		delete(vars.StreamState_DBs, host+"/"+container)
	}
}

func getExistingDB(host, container, dbType string) *leveldb.DB {
	switch dbType {
	case "logs":
		return vars.ActiveDBs[host+"/"+container]
	case "statistics":
		return vars.Stat_Containers_DBs[host+"/"+container]
	case "hosts_statistics":
		return vars.Stat_Hosts_DBs[host]
	case "statuses":
		return vars.Statuses_DBs[host+"/"+container]
	case "brokenlogs":
		return vars.BrokenLogs_DBs[host+"/"+container]
	case "containersmeta":
		return vars.ContainersMeta_DBs[host+"/"+container]
	case "streamstate":
		return vars.StreamState_DBs[host+"/"+container]
	}
	return nil
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

func GetHost() string {
	hostname, err := os.ReadFile("/etc/hostname")
	return parseHostname(string(hostname), err)
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
	c, _ := req.Cookie("onlogs-cookie")
	if c == nil {
		return "", errors.New("401 - Unauthorized!")
	}

	// An empty key verifies every HMAC token.
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", errors.New("401 - Unauthorized!")
	}

	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(c.Value, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())

	if err != nil {
		return "", err
	}

	exp, ok := claims["exp"].(float64)
	if !ok {
		return "", errors.New("Token is expired")
	}
	if int64(exp) < time.Now().Unix() {
		return "", errors.New("Token is expired")
	}

	user, ok := claims["user"].(string)
	if !ok || user == "" {
		return "", errors.New("401 - Unauthorized!")
	}

	// A valid signature is not enough: deleting an account must revoke it.
	if exists, err := vars.UsersDB.Has([]byte(user), nil); err != nil || !exists {
		return "", errors.New("401 - Unauthorized!")
	}
	return user, nil
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

func DeleteDockerLogs(host string, container string) error {
	if host != GetHost() {
		vars.QueueDelete(host, container)
		return nil
	}

	containerID := GetDockerContainerID(host, container)
	files, err := os.ReadDir("/var/lib/docker/containers/" + containerID)
	if err != nil || len(files) == 0 {
		return err
	}

	for _, file := range files {
		if file.Name() == containerID+"-json.log" {
			os.WriteFile("/var/lib/docker/containers/"+containerID+"/"+containerID+"-json.log", nil, 0640)
		}
	}

	return nil
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

// func RunSpaceMonitoring() {
// 	for {
// 		to_put, _ := json.Marshal(GetStorageData())
// 		vars.StateDB.Put([]byte("Storage data"), to_put, nil)

// 		time.Sleep(time.Second * 30)
// 	}
// }

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
