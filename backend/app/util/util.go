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

func CreateJWT() string {
	token := jwt.New(jwt.SigningMethodEdDSA)
	claims := token.Claims.(jwt.MapClaims)
	claims["exp"] = time.Now().Add(10 * time.Minute)
	claims["authorized"] = true
	claims["user"] = "username"
	tokenString, _ := token.SignedString(os.Getenv("JWT_TOKEN")) // need to store it to env var

	return tokenString
}
