package util

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devforth/OnLogs/app/vars"
	"github.com/golang-jwt/jwt"
	"github.com/syndtr/goleveldb/leveldb"
	"golang.org/x/sys/unix"
)

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

func CreateJWT(login string) string {
	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	claims["exp"] = time.Now().AddDate(0, 0, 2).Unix()
	claims["authorized"] = true
	claims["user"] = login
	tokenString, _ := token.SignedString([]byte(os.Getenv("JWT_SECRET")))

	return tokenString
}

func GetDB(host string, container string, dbType string) *leveldb.DB {
	var res_db *leveldb.DB
	if dbType == "logs" {
		res_db = vars.ActiveDBs[container]
	} else if dbType == "statuses" {
		res_db = vars.Statuses_DBs[host+"/"+container]
	} else if dbType == "statistics" {
		res_db = vars.Stat_Containers_DBs[host+"/"+container]
	}

	var err error
	if res_db == nil {
		path := "leveldb/hosts/" + host + "/containers/" + container + "/" + dbType
		res_db, err = leveldb.OpenFile(path, nil)
		if err != nil {
			res_db, _ = leveldb.RecoverFile(path, nil)
		}
	}
	return res_db
}

func GetHost() string {
	hostname, err := os.ReadFile("/etc/hostname")
	var host string
	if err != nil {
		host, _ = os.Hostname()
	} else {
		host = string(hostname)
	}

	if host[len(host)-1] < 32 || host[len(host)-1] > 126 {
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

	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
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

func GenerateJWTSecret() string {
	tokenLen := 25

	letterBytes := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890-_."
	b := make([]byte, tokenLen)

	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	for i := range b {
		b[i] = letterBytes[r1.Int63()%int64(len(letterBytes))]
	}
	token := string(b)

	return token
}

func GetDockerContainerID(host string, container string) string {
	_, err := os.ReadDir("leveldb/hosts/" + host)
	if err != nil {
		return ""
	}

	idDB, _ := leveldb.OpenFile("leveldb/hosts/"+host+"/containersMeta", nil)
	defer idDB.Close()
	iter := idDB.NewIterator(nil, nil)
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
		vars.ToDelete[host] = append(vars.ToDelete[host+"/"+container], container)
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
