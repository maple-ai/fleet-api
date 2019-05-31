package api

import (
	"net/http"

	"github.com/gorilla/context"
	"github.com/maple-ai/syrup"
	"gopkg.in/mgo.v2/bson"
	"github.com/maple-ai/fleet-api/db"
)

func adminGetGarage(w http.ResponseWriter, r *http.Request) {
	syrup.WriteJSON(w, http.StatusOK, context.Get(r, "garage"))
}

func adminGetGarages(w http.ResponseWriter, r *http.Request) {
	var garages []db.Garage
	if err := db.Cols.Garages.Find(db.M{}).All(&garages); err != nil {
		panic(err)
	}

	syrup.WriteJSON(w, http.StatusOK, garages)
}

func adminSaveGarage(w http.ResponseWriter, r *http.Request) {
	var garage db.Garage
	if err := syrup.Bind(w, r, &garage); err != nil {
		return
	}

	if r.Method == "POST" {
		garage.ID = bson.NewObjectId()
		if err := db.Cols.Garages.Insert(&garage); err != nil {
			panic(err)
		}

		syrup.WriteJSON(w, http.StatusCreated, garage)
		return
	}

	garageID := context.Get(r, "garage").(db.Garage).ID
	garage.ID = garageID
	if err := db.Cols.Garages.UpdateId(garageID, db.M{"$set": &garage}); err != nil {
		panic(err)
	}

	syrup.WriteJSON(w, http.StatusOK, garage)
}

func adminDeleteGarage(w http.ResponseWriter, r *http.Request) {
	garage := context.Get(r, "garage").(db.Garage)
	if err := db.Cols.Garages.RemoveId(garage.ID); err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusNoContent)
}
