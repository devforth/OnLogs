package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	routes "github.com/devforth/OnLogs/app/routes"
	util "github.com/devforth/OnLogs/app/util"
	"github.com/tv42/httpunix"
)

type Client struct {
}

func get_containers_names_list(w http.ResponseWriter, req *http.Request) {
	unix := &httpunix.Transport{
		DialTimeout:           100 * time.Millisecond,
		RequestTimeout:        1 * time.Second,
		ResponseHeaderTimeout: 1 * time.Second,
	}
	unix.RegisterLocation("daemon", "/var/run/docker.sock")

	var client = http.Client{
		Transport: unix,
	}

	resp, _ := client.Get("http+unix://daemon/containers/json")
	body, _ := ioutil.ReadAll(resp.Body)
	var result []map[string]any
	var names []string
	json.Unmarshal(body, &result)
	for i := 0; i < len(result); i++ {
		for key, element := range result[i] {
			if key == "Names" {
				name := element.([]interface{})[0].(string)
				str := fmt.Sprintf("%v", name)
				names = append(names, string(str))
			}
		}
	}
	json.NewEncoder(w).Encode(names)
	resp.Body.Close()
}

func main() {
	util.StoreLogs() // store logs from all containers before getting started

	http.HandleFunc("api/v1/getHost", routes.RouteGetHost)
	http.HandleFunc("api/v1/getLogs", routes.RouteGetContainerLogs)

	http.ListenAndServe(":2874", nil)
}
