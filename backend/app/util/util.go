package util

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devforth/OnLogs/app/vars"
	"github.com/golang-jwt/jwt"
	"github.com/syndtr/goleveldb/leveldb"
)

func Contains(a string, list []string) bool {
	for _, b := range list {
		if strings.Compare(b, a) == 0 {
			return true
		}
	}
	return false
}

func CreateInitUser() {
	vars.UsersDB.Put([]byte("admin"), []byte(os.Getenv("PASSWORD")), nil)
}

func restartStats(host string, container string, current_db *leveldb.DB) {
	var used_storage map[string]map[string]int
	var location string
	if container == "" {
		used_storage = vars.Counters_For_Hosts_Last_30_Min
		location = host
	} else {
		used_storage = vars.Counters_For_Containers_Last_30_Min
		location = host + "/" + container
	}
	copy := map[string]int{"error": 0, "debug": 0, "info": 0, "warn": 0, "other": 0}
	copy["error"] = used_storage[location]["error"]
	copy["debug"] = used_storage[location]["debug"]
	copy["info"] = used_storage[location]["info"]
	copy["warn"] = used_storage[location]["warn"]
	copy["other"] = used_storage[location]["other"]
	to_put, _ := json.Marshal(copy)
	datetime := strings.Replace(strings.Split(time.Now().UTC().String(), ".")[0], " ", "T", 1) + "Z"
	current_db.Put([]byte(datetime), to_put, nil)

	used_storage[location]["error"] = 0
	used_storage[location]["debug"] = 0
	used_storage[location]["info"] = 0
	used_storage[location]["warn"] = 0
	used_storage[location]["other"] = 0
}

func RunStatisticForContainer(host string, container string) {
	location := host + "/" + container
	vars.Counters_For_Containers_Last_30_Min[location] = map[string]int{"error": 0, "debug": 0, "info": 0, "warn": 0, "other": 0}
	if vars.Stat_Containers_DBs[location] == nil {
		current_db, _ := leveldb.OpenFile("leveldb/hosts/"+host+"/containers/"+container+"/statistics", nil)
		defer current_db.Close()
		vars.Stat_Containers_DBs[location] = current_db
	}
	defer delete(vars.Stat_Containers_DBs, location)
	defer restartStats(host, container, vars.Stat_Containers_DBs[location])
	for {
		vars.Counters_For_Hosts_Last_30_Min[host]["error"] += vars.Counters_For_Containers_Last_30_Min[location]["error"]
		vars.Counters_For_Hosts_Last_30_Min[host]["debug"] += vars.Counters_For_Containers_Last_30_Min[location]["debug"]
		vars.Counters_For_Hosts_Last_30_Min[host]["info"] += vars.Counters_For_Containers_Last_30_Min[location]["info"]
		vars.Counters_For_Hosts_Last_30_Min[host]["warn"] += vars.Counters_For_Containers_Last_30_Min[location]["warn"]
		vars.Counters_For_Hosts_Last_30_Min[host]["other"] += vars.Counters_For_Containers_Last_30_Min[location]["other"]
		restartStats(host, container, vars.Stat_Containers_DBs[location])
		time.Sleep(30 * time.Minute)
	}
}

// TODO improve counters
func RunStatisticForHost(host string) {
	vars.Counters_For_Hosts_Last_30_Min[host] = map[string]int{"error": 0, "debug": 0, "info": 0, "warn": 0, "other": 0}
	if vars.Stat_Hosts_DBs[host] == nil {
		current_db, _ := leveldb.OpenFile("leveldb/hosts/"+host+"/statistics", nil)
		defer current_db.Close()
		vars.Stat_Hosts_DBs[host] = current_db
	}
	defer delete(vars.Stat_Hosts_DBs, host)
	defer restartStats(host, "", vars.Stat_Hosts_DBs[host])
	for {
		restartStats(host, "", vars.Stat_Hosts_DBs[host])
		time.Sleep(30 * time.Minute)
	}
}

func replaceVarForAllFilesInDir(dirName string, dir_files []fs.DirEntry) {
	for _, dir_file := range dir_files {
		if strings.HasSuffix(dir_file.Name(), ".js") || strings.HasSuffix(dir_file.Name(), ".css") || strings.HasSuffix(dir_file.Name(), ".html") {
			input, err := ioutil.ReadFile("dist/" + dirName + "/" + dir_file.Name())
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			output := bytes.Replace(input, []byte("/ONLOGS_PREFIX_ENV_VARIABLE_THAT_SHOULD_BE_REPLACED_ON_BACKEND_INITIALIZATION"), []byte(os.Getenv("ONLOGS_PATH_PREFIX")), -1)
			if err = ioutil.WriteFile("dist/"+dirName+"/"+dir_file.Name(), output, 0666); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}
	}
}

func ReplacePrefixVariableForFrontend() {
	files, err := os.ReadDir("dist")
	if err != nil {
		fmt.Println("INFO: unable to find 'dist' folder")
		return
	}
	fmt.Println("INFO: base onlogs prefix is: ", "\""+os.Getenv("ONLOGS_PATH_PREFIX")+"\"")
	for _, file := range files {
		if file.IsDir() {
			dir_files, _ := os.ReadDir("dist/" + file.Name())
			replaceVarForAllFilesInDir(file.Name(), dir_files)
		}
	}
	replaceVarForAllFilesInDir("", files)
}

func SendInitRequest() {
	postBody, _ := json.Marshal(map[string]string{
		"Hostname": GetHost(),
		"Token":    os.Getenv("ONLOGS_TOKEN"),
	})
	responseBody := bytes.NewBuffer(postBody)

	resp, err := http.Post(os.Getenv("HOST")+"/api/v1/addHost", "application/json", responseBody)
	if err != nil {
		panic("ERROR: Can't send request to host!\n" + err.Error())
	}

	if resp.StatusCode != 200 {
		b, _ := ioutil.ReadAll(resp.Body)
		panic("ERROR: Response status from host is " + resp.Status + "\nResponse body: " + string(b))
	}
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

func GetHost() string {
	hostname, err := os.ReadFile("/etc/hostname")
	var host string
	if err != nil {
		host, _ = os.Hostname()
	} else {
		host = string(hostname)
	}

	if []byte(host)[0] < 32 || []byte(host)[0] < 126 {
		host = host[:len(host)-1]
	}
	return host
}

func GetDirSize(host string, container string) float64 {
	var size int64

	path := "leveldb/hosts/" + host + "/containers/" + container
	_, pathErr := os.Stat(path)
	if os.IsNotExist(pathErr) {
		return 0
	}

	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	if err != nil {
		fmt.Println(err)
	}

	return float64(size) / (1024.0 * 1024.0)
}

func GetUserFromJWT(req http.Request) (string, error) {
	c, _ := req.Cookie("onlogs-cookie")
	if c == nil {
		return "", errors.New("401 - Unauthorized!")
	}

	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(c.Value, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})

	if err != nil && strings.Compare(err.Error(), "Token is expired") != 0 {
		return "", err
	}

	if int64(int64(claims["exp"].(float64))) < time.Now().Unix() {
		return "", errors.New("Token is expired")
	}

	return claims["user"].(string), nil
}
