package routes

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"time"

	daemon "github.com/devforth/OnLogs/app/daemon"
	"github.com/devforth/OnLogs/app/db"
	"github.com/devforth/OnLogs/app/srchx_db"
	vars "github.com/devforth/OnLogs/app/vars"
	"github.com/golang-jwt/jwt"
)

func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Credentials", "true")
	(*w).Header().Set("Access-Control-Allow-Methods", "*")
	(*w).Header().Set("Access-Control-Allow-Headers", "*")
}

func createJWT() string {
	token := jwt.New(jwt.SigningMethodEdDSA)
	claims := token.Claims.(jwt.MapClaims)
	claims["exp"] = time.Now().Add(10 * time.Minute)
	claims["authorized"] = true
	claims["user"] = "username"
	tokenString, _ := token.SignedString("srakapopa") // need to store it to env var

	return tokenString
}

func RouteGetHost(w http.ResponseWriter, req *http.Request) {
	enableCors(&w)
	host, _ := os.Hostname()
	to_return := &vars.HostsList{Host: host, Services: daemon.GetContainersList()}
	e, _ := json.Marshal(to_return)
	w.Write(e)
}

func RouteGetLogs(w http.ResponseWriter, req *http.Request) {
	params := req.URL.Query()
	limit, _ := strconv.Atoi(params.Get("limit"))
	offset, _ := strconv.Atoi(params.Get("offset"))
	json.NewEncoder(w).Encode(srchx_db.GetLogs(params.Get("id"), params.Get("search"), limit, offset))
}

func RouteLogin(w http.ResponseWriter, req *http.Request) {
	if req.Method == "POST" {
		var loginData vars.UserData
		decoder := json.NewDecoder(req.Body)
		decoder.Decode(&loginData)

		isCorrect := db.CheckUserPassword(loginData.Login, loginData.Password)
		if !isCorrect {
			json.NewEncoder(w).Encode(map[string]string{"error": "Wrong login or password!"})
		}
		http.SetCookie(w, &http.Cookie{
			Name:    "onlogs-cookie",
			Value:   createJWT(),
			Expires: time.Now().AddDate(0, 0, 2),
		})
	}
}

func RouteCreateUser(w http.ResponseWriter, req *http.Request) {
	if req.Method == "POST" {
		var loginData vars.UserData
		decoder := json.NewDecoder(req.Body)
		decoder.Decode(&loginData)

		err := db.CreateUser(loginData.Login, loginData.Password)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		}
		http.SetCookie(w, &http.Cookie{
			Name:    "onlogs-cookie",
			Value:   createJWT(),
			Expires: time.Now().AddDate(0, 0, 2),
		})
	}
}
