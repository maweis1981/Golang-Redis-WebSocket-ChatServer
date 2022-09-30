package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"os"
	"tower/api"
	"tower/rdcon"
)

const (
	// used to track users that used chat. mainly for listing users in the /users api, in real world chat app
	// such devices list should be separated into devices management module.
	usersKey       = "users"
	userChannelFmt = "devices:%s:channels"
	ChannelsKey    = "channels"
)

func main() {

	rdcon.GetRedis().Client.SAdd(ChannelsKey, "general", "random")

	r := mux.NewRouter()

	r.Path("/ws/{device}").Methods("GET").HandlerFunc(api.DeviceWebSocketHandler)

	r.Path("/devices/{device}/channels").Methods("GET").HandlerFunc(api.DeviceChannelsHandler)
	r.Path("/devices").Methods("GET").HandlerFunc(api.DeviceHandler)

	port := ":" + os.Getenv("PORT")
	if port == ":" {
		port = "0.0.0.0:8000"
	}
	fmt.Println("chat service started on port", port)
	log.Fatal(http.ListenAndServe(port, r))
}
