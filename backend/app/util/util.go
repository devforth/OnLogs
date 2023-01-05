package util

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devforth/OnLogs/app/vars"
	"github.com/golang-jwt/jwt"
)

func Contains(a string, list []string) bool {
	for _, b := range list {
		if strings.Compare(b, a) == 0 {
			return true
		}
	}
	return false
}

func CreateOnLogsToken() string {
	tokenLen := 25
	content, _ := ioutil.ReadFile("leveldb/onLogsToken")
	if len(content) == tokenLen {
		return ""
	}

	file, _ := os.Create("leveldb/onLogsToken")
	letterBytes := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890!@#$%^&*()_+,.:'{}[]"
	b := make([]byte, tokenLen)

	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	for i := range b {
		b[i] = letterBytes[r1.Int63()%int64(len(letterBytes))]
	}
	token := string(b)

	file.WriteString(token)
	return token
}

func CreateInitUser() {
	vars.UsersDB.Put([]byte("admin"), []byte(os.Getenv("PASSWORD")), nil)
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
		fmt.Println(string(b))
		panic("ERROR: Response status from host is " + resp.Status) // TODO: Improve text with host response body
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

func GetOnLogsToken() string {
	tokenLen := 25
	content, _ := ioutil.ReadFile("leveldb/onLogsToken")
	if len(content) != tokenLen {
		return CreateOnLogsToken()
	}
	return string(content)
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
	var path string
	if host != GetHost() && host != "" {
		path = "leveldb/hosts/" + host + "/" + container
	} else {
		path = "leveldb/logs/" + container
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
