package api

import (
	"net/http"
	"time"

	"gopkg.in/mgo.v2/bson"

	"github.com/gorilla/context"
	"github.com/gorilla/mux"
	"github.com/maple-ai/syrup"
	"github.com/maple-ai/fleet-api/db"
)

func adminGetShiftCalendar(w http.ResponseWriter, r *http.Request) {
	todayStart, err := time.Parse("02-01-2006", r.URL.Query().Get("day"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var results []db.M
	if err := db.Cols.Shifts.Pipe([]db.M{
		{"$match": db.M{
			"date": db.M{
				"$gte": todayStart,
				"$lt":  todayStart.Add(24 * time.Hour),
			},
		}},
		{"$lookup": db.M{
			"from":         "bikes",
			"localField":   "scooter_id",
			"foreignField": "_id",
			"as":           "scooter",
		}},
		{"$lookup": db.M{
			"from":         "users",
			"localField":   "user_id",
			"foreignField": "_id",
			"as":           "user",
		}},
		{"$match": db.M{
			"user.0": db.M{"$exists": true},
		}},
		{"$sort": db.M{
			"date": 1,
		}},
		{"$unwind": "$scooter"},
		{"$group": db.M{
			"_id":    "$scooter_id",
			"shifts": db.M{"$push": "$$ROOT"},
		}},
	}).All(&results); err != nil {
		panic(err)
	}

	syrup.WriteJSON(w, http.StatusOK, &results)
}

func adminGetShiftInfo(w http.ResponseWriter, r *http.Request) {
	shiftID := bson.ObjectIdHex(mux.Vars(r)["shift_id"])

	var shift db.M
	if err := db.Cols.Shifts.Pipe([]db.M{
		{"$match": db.M{
			"_id": shiftID,
		}},
		{"$lookup": db.M{
			"from":         "bikes",
			"localField":   "scooter_id",
			"foreignField": "_id",
			"as":           "bike",
		}},
		{"$lookup": db.M{
			"from":         "users",
			"localField":   "user_id",
			"foreignField": "_id",
			"as":           "user",
		}},
	}).One(&shift); err != nil {
		panic(err)
	}

	syrup.WriteJSON(w, http.StatusOK, shift)
}

func adminShiftCheckIn(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Date time.Time `json:"date"`
	}
	if err := syrup.Bind(w, r, &body); err != nil {
		return
	}

	shiftID := bson.ObjectIdHex(mux.Vars(r)["shift_id"])
	if body.Date.IsZero() {
		body.Date = time.Now()
	}

	var shift db.Shift
	if err := db.Cols.Shifts.FindId(shiftID).One(&shift); err != nil {
		panic(err)
	}

	if err := db.Cols.Shifts.UpdateId(shiftID, db.M{
		"$set": db.M{
			"check_in":          body.Date,
			"check_in_operator": context.Get(r, "userID").(bson.ObjectId),
			"status":            "running",
		},
	}); err != nil {
		panic(err)
	}

	syrup.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"check_in": body.Date,
		"status":   "running",
	})
}

func adminShiftCheckOut(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Date time.Time `json:"date"`
	}
	if err := syrup.Bind(w, r, &body); err != nil {
		return
	}

	shiftID := bson.ObjectIdHex(mux.Vars(r)["shift_id"])
	if body.Date.IsZero() {
		body.Date = time.Now()
	}

	var shift db.Shift
	if err := db.Cols.Shifts.FindId(shiftID).One(&shift); err != nil {
		panic(err)
	}

	if shift.CheckIn.IsZero() == true {
		// Not checked in
		syrup.WriteJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Not checked in",
		})
		return
	}

	// round up shift end
	minute := body.Date.Minute()
	if rem := minute % 15; rem > 0 {
		body.Date = body.Date.Add(time.Duration(15-rem) * time.Minute)
	}

	if err := db.Cols.Shifts.UpdateId(shiftID, db.M{
		"$set": db.M{
			"check_out":          body.Date,
			"check_out_operator": context.Get(r, "userID").(bson.ObjectId),
			"status":             "complete",
		},
	}); err != nil {
		panic(err)
	}

	syrup.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"check_out": body.Date,
		"status":    "complete",
	})
}

func adminShiftReset(w http.ResponseWriter, r *http.Request) {
	shiftID := bson.ObjectIdHex(mux.Vars(r)["shift_id"])
	if err := db.Cols.Shifts.UpdateId(shiftID, db.M{
		"$set": db.M{
			"status": "created",
		},
		"$unset": db.M{
			"check_in":           1,
			"check_out":          1,
			"check_in_operator":  1,
			"check_out_operator": 1,
		},
	}); err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusNoContent)
}

func adminShiftNotes(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Notes string `json:"notes"`
	}

	if err := db.Cols.Shifts.UpdateId(bson.ObjectIdHex(mux.Vars(r)["shift_id"]), db.M{
		"$set": db.M{"notes": body.Notes},
	}); err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusNoContent)
}

func adminApproveShiftStatus(w http.ResponseWriter, r *http.Request) {
	shiftID := bson.ObjectIdHex(mux.Vars(r)["shift_id"])
	var shift db.Shift
	if err := db.Cols.Shifts.FindId(shiftID).One(&shift); err != nil {
		panic(err)
	}

	deleteSet := db.M{
		"status":       "cancelled",
		"deleted":      true,
		"deleted_date": time.Now(),
		"deleted_by":   context.Get(r, "userID"),
	}

	if r.Method == "POST" {
		day, _ := time.Parse("02-01-2006", shift.Date.Format("02-01-2006"))
		db.Cols.Shifts.UpdateAll(db.M{
			"date": db.M{
				"$gte": day,
				"$lt":  day.Add(24 * time.Hour),
			},
			"_id": db.M{
				"$ne": shiftID,
			},
			"scooter_id": shift.ScooterID,
			"status":     "created",
		}, db.M{"$set": deleteSet})

		db.Cols.Shifts.UpdateId(shiftID, db.M{"$set": db.M{
			"status": "confirmed",
		}})
	} else {
		db.Cols.Shifts.UpdateId(shiftID, db.M{"$set": deleteSet})
	}

	w.WriteHeader(http.StatusNoContent)
}

func adminReassignBike(w http.ResponseWriter, r *http.Request) {
	shiftID := bson.ObjectIdHex(mux.Vars(r)["shift_id"])
	var shift db.Shift
	if err := db.Cols.Shifts.FindId(shiftID).One(&shift); err != nil {
		panic(err)
	}

	bikeID := bson.ObjectIdHex(mux.Vars(r)["bike_id"])
	db.Cols.Shifts.UpdateId(shift.ID, db.M{"$set": db.M{
		"deleted":    false,
		"status":     "confirmed",
		"scooter_id": bikeID,
	}})

	w.WriteHeader(http.StatusNoContent)
}
