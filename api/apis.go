/**
* API Handler
*
 */

package api

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"net/http"
	"tower/devices"
)

// DeviceChannelsHandler Path: /devices/{device}/channels
func DeviceChannelsHandler(w http.ResponseWriter, r *http.Request) {
	deviceToken := mux.Vars(r)["device"]

	list, err := devices.GetChannels(deviceToken)
	if err != nil {
		handleError(err, w)
		return
	}
	err = json.NewEncoder(w).Encode(list)
	if err != nil {
		handleError(err, w)
		return
	}
}

// DeviceHandler Path: /devices
func DeviceHandler(w http.ResponseWriter, r *http.Request) {

	list, err := devices.List()
	if err != nil {
		handleError(err, w)
		return
	}
	err = json.NewEncoder(w).Encode(list)
	if err != nil {
		handleError(err, w)
		return
	}
}

func handleError(err error, w http.ResponseWriter) {
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(fmt.Sprintf(`{"err": "%s"}`, err.Error())))
}
