package api

import (
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gorilla/mux"
	"gopkg.in/mgo.v2/bson"
	"github.com/maple-ai/fleet-api/config"
	"github.com/maple-ai/fleet-api/db"
)

func adminGetGPSPositions(w http.ResponseWriter, r *http.Request) {
	var shift db.Shift
	if err := db.Cols.Shifts.FindId(bson.ObjectIdHex(mux.Vars(r)["shift_id"])).One(&shift); err != nil {
		panic(err)
	}

	var bike db.Bike
	db.Cols.Bikes.FindId(shift.ScooterID).One(&bike)

	if bike.TrackerID <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	q := url.Values{}
	q.Set("deviceId", strconv.Itoa(bike.TrackerID))
	q.Set("from", shift.CheckIn.Format("2006-01-02T15:04:05.000Z"))
	q.Set("to", shift.CheckOut.Format("2006-01-02T15:04:05.000Z"))
	q.Set("page", "1")
	q.Set("start", "0")
	q.Set("limit", "25")

	req, _ := http.NewRequest("GET", config.Config.GPS.Endpoint+"/positions?"+q.Encode(), nil)
	req.Header.Add("Authorization", config.GetGPSAuthorization())

	if resp, err := (&http.Client{}).Do(req); err != nil {
		panic(err)
	} else {
		defer resp.Body.Close()

		w.WriteHeader(http.StatusOK)
		io.Copy(w, resp.Body)
	}
}
