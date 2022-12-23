package util

import (
	"errors"
	"io/ioutil"
	"net/http"
	"os"
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

func CreateInitUser() {
	vars.UsersDB.Put([]byte("admin"), []byte(os.Getenv("PASSWORD")), nil)
}

func CreateInitHost() {
	_, err := ioutil.ReadFile("leveldb/hosts/hostsList")
	if err != nil {
		os.MkdirAll("leveldb/hosts", 0700)
		hostname, err := os.ReadFile("/etc/hostname")
		var host string
		if err != nil {
			host, _ = os.Hostname()
		} else {
			host = string(hostname)
		}
		os.WriteFile("leveldb/hosts/hostsList", []byte(host+"\n"), 0777)
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
