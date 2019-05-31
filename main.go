package main

import (
	"fmt"
	"net/http"

	"github.com/maple-ai/fleet-api/api"
	"github.com/maple-ai/fleet-api/config"
	"github.com/maple-ai/fleet-api/db"
)

func main() {
	if err := config.Parse(); err != nil {
		panic(err)
	}

	if err := db.Connect(); err != nil {
		panic(err)
	}

	db.Setup()

	http.Handle("/", api.Routes())

	fmt.Println("Listening on port " + config.Config.Port)
	http.ListenAndServe(config.Config.Port, nil)
}
