package util

import (
	"os"
	"time"

	"github.com/devforth/OnLogs/app/vars"
	"github.com/golang-jwt/jwt"
)

func CreateInitUser() {
	vars.UsersDB.Put([]byte("admin"), []byte(os.Getenv("PASSWORD")), nil)
}

func CreateJWT(login string) string {
	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	claims["exp"] = time.Now().Add(10 * time.Minute)
	claims["authorized"] = true
	claims["user"] = login
	tokenString, _ := token.SignedString([]byte(os.Getenv("JWT_TOKEN")))

	return tokenString
}
