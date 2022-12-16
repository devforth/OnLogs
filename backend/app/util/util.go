package util

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/devforth/OnLogs/app/vars"
	"github.com/golang-jwt/jwt"
)

func RemoveOldFiles() {
	os.RemoveAll("leveldb") // may cause crashes
	os.RemoveAll("onlogsdb")
	files, _ := os.ReadDir("logDump")
	for _, name := range files {
		os.RemoveAll("logDump/" + name.Name())
	}
}

func StartLogDumpGarbageCollector() {
	for {
		time.Sleep(1 * time.Minute)
		containersDump, _ := os.ReadDir("logDump")
		for _, containerDump := range containersDump {
			dumpFiles, _ := os.ReadDir(containerDump.Name())
			for _, dumpFile := range dumpFiles {
				if strings.HasSuffix(dumpFile.Name(), ".bad") {
					go os.Remove("logDump/" + containerDump.Name() + "/" + dumpFile.Name())
				}
			}
		}
	}
}

func CreateInitUser() {
	vars.UsersDB.Put([]byte("admin"), []byte(os.Getenv("PASSWORD")), nil)
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
