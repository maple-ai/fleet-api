package api

import (
	"net/http"

	"github.com/gorilla/context"
	"github.com/gorilla/mux"
	"gopkg.in/mgo.v2/bson"
	"github.com/maple-ai/fleet-api/db"
)

func adminBikeMiddleware(w http.ResponseWriter, r *http.Request) {
	bikeID := bson.ObjectIdHex(mux.Vars(r)["bike_id"])

	if !bikeID.Valid() {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var bike db.Bike
	if err := db.Cols.Bikes.FindId(bikeID).One(&bike); err != nil {
		panic(err)
	}

	context.Set(r, "bike", bike)
}

func adminGarageMiddleware(w http.ResponseWriter, r *http.Request) {
	garageID := bson.ObjectIdHex(mux.Vars(r)["garage_id"])

	if !garageID.Valid() {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var garage db.Garage
	if err := db.Cols.Garages.FindId(garageID).One(&garage); err != nil {
		panic(err)
	}

	context.Set(r, "garage", garage)
}

func adminUserMiddleware(w http.ResponseWriter, r *http.Request) {
	userID := bson.ObjectIdHex(mux.Vars(r)["user_id"])

	if !userID.Valid() {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var user db.User
	if err := db.Cols.Users.FindId(userID).One(&user); err != nil {
		panic(err)
	}

	context.Set(r, "admin_user", user)
}
