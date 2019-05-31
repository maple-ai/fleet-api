package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/context"
	"github.com/gorilla/mux"
	"github.com/maple-ai/syrup"
	"gopkg.in/mgo.v2/bson"
	"github.com/maple-ai/fleet-api/db"
)

func shiftMiddleware(w http.ResponseWriter, r *http.Request) {
	userID := context.Get(r, "userID").(bson.ObjectId)
	if adminUserObj, ok := context.GetOk(r, "admin_user"); ok {
		// comes through admin
		userID = adminUserObj.(db.User).ID
	}

	var shift db.Shift
	if shiftID := bson.ObjectIdHex(mux.Vars(r)["shift_id"]); shiftID.Valid() == false {
		w.WriteHeader(http.StatusBadRequest)
	} else if err := db.Cols.Shifts.Find(db.M{
		"_id":     shiftID,
		"user_id": userID,
	}).One(&shift); err != nil {
		panic(err)
	}

	context.Set(r, "shift", shift)
}

func getShifts(w http.ResponseWriter, r *http.Request) {
	var start, end time.Time
	query := r.URL.Query()
	userID := context.Get(r, "userID").(bson.ObjectId)
	if adminUserObj, ok := context.GetOk(r, "admin_user"); ok {
		// comes through admin
		userID = adminUserObj.(db.User).ID
	}

	if year, err := strconv.Atoi(query.Get("year")); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	} else if month, err := strconv.Atoi(query.Get("month")); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	} else {
		start = time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Now().Location())
		end = time.Date(year, time.Month(month+1), 1, 0, 0, 0, 0, time.Now().Location())
	}

	var shifts []db.M
	if err := db.Cols.Shifts.Pipe([]db.M{
		db.M{"$match": db.M{
			"user_id": userID,
			"date": db.M{
				"$gte": start,
				"$lt":  end,
			},
			// "deleted": false,
		}},
		db.M{"$lookup": db.M{
			"from":         "bikes",
			"localField":   "scooter_id",
			"foreignField": "_id",
			"as":           "bike",
		}},
		db.M{"$unwind": "$bike"},
		db.M{"$sort": db.M{"date": 1}},
	}).All(&shifts); err != nil {
		panic(err)
	}

	syrup.WriteJSON(w, http.StatusOK, shifts)
}

func createShift(w http.ResponseWriter, r *http.Request) {
	var shift struct {
		GarageID bson.ObjectId `json:"garage_id"`
		BikeID   bson.ObjectId `json:"bike_id"`
		Date     string        `json:"date"`
		Hour     int           `json:"hour"`
	}
	if err := syrup.Bind(w, r, &shift); err != nil {
		return
	}

	errs := []string{}
	var shiftDate time.Time

	_, isAdmin := context.GetOk(r, "is_admin")

	// check garage ID
	if shift.GarageID.Valid() == false {
		errs = append(errs, "Garage does not exist")
	} else if count, err := db.Cols.Garages.FindId(shift.GarageID).Count(); err != nil {
		panic(err)
	} else if count == 0 {
		errs = append(errs, "Garage does not exist")
	}

	// validate date
	if date, err := time.Parse("02-01-2006", shift.Date); err != nil {
		errs = append(errs, "Date: invalid format (must be DD-MM-YYYY)")
	} else if date.Add(time.Hour*24).Before(time.Now()) && !isAdmin {
		errs = append(errs, "Date: must be in the future")
	} else {
		shiftDate = date.Add(time.Duration(shift.Hour) * time.Hour)

		// Check local timezone. Parsed & stored in UTC, but local time can be other timezone.
		currentLocation, _ := time.ParseInLocation("2006-01-02 15:04", shiftDate.Format("2006-01-02 15:04"), time.Now().Location())
		if currentLocation.Before(time.Now()) && !isAdmin {
			errs = append(errs, "Date: cannot book a past shift")
		}
	}

	// Validate hour
	if len(errs) > 0 {
		syrup.WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
			"errors": errs,
		})
		return
	}

	// Get available scooters
	bikes := db.GetAvailableScooters(shift.GarageID, shiftDate, 0)
	var foundBike *db.Bike

	for _, bike := range bikes {
		if bike.ID.Hex() == shift.BikeID.Hex() {
			foundBike = &bike
			break
		}
	}

	if foundBike == nil {
		w.WriteHeader(http.StatusConflict)
		return
	}

	userID := context.Get(r, "userID").(bson.ObjectId)
	if adminUserObj, ok := context.GetOk(r, "admin_user"); ok {
		// comes through admin
		userID = adminUserObj.(db.User).ID
	}

	shiftDoc := db.Shift{
		GarageID:  shift.GarageID,
		UserID:    userID,
		ScooterID: foundBike.ID,

		Status: "created",
		Date:   shiftDate,

		Added: time.Now(),
		// Note, userID might not be same as context.Get(r, "userID") (admin call)
		AddedBy: context.Get(r, "userID").(bson.ObjectId),
	}
	if err := db.Cols.Shifts.Insert(&shiftDoc); err != nil {
		panic(err)
	}

	syrup.WriteJSON(w, http.StatusCreated, shiftDoc)
}

func shiftSearch(w http.ResponseWriter, r *http.Request) {
	date, err := time.Parse("02-01-2006", r.URL.Query().Get("date"))
	if err != nil || date.Add(time.Hour*24).Before(time.Now()) {
		if _, ok := context.GetOk(r, "is_admin"); !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	// check garage ID
	garageID := bson.ObjectIdHex(r.URL.Query().Get("garage"))
	if garageID.Valid() == false {
		w.WriteHeader(http.StatusBadRequest)
	} else if count, err := db.Cols.Garages.FindId(garageID).Count(); err != nil {
		panic(err)
	} else if count == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	userID := context.Get(r, "userID").(bson.ObjectId)
	if adminUserObj, ok := context.GetOk(r, "admin_user"); ok {
		// comes through admin
		userID = adminUserObj.(db.User).ID
	}

	var membership db.UserMembership
	db.Cols.Memberships.Find(db.M{"user_id": userID}).One(&membership)

	maxCC := 0
	if membership.License == "cbt" {
		maxCC = 125
	}

	syrup.WriteJSON(w, http.StatusOK, db.GetAvailableScooters(garageID, date, maxCC))
}

func cancelShift(w http.ResponseWriter, r *http.Request) {
	shift := context.Get(r, "shift").(db.Shift)
	errs := []string{}

	if shift.Deleted {
		errs = append(errs, "Shift already deleted")
	}

	if shift.Date.UTC().Before(time.Now()) || shift.CheckIn.IsZero() == false {
		// shift in progress or in the past
		errs = append(errs, "Shift in the past or you have been checked in")
	}

	if len(errs) > 0 {
		syrup.WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
			"errors": errs,
		})
		return
	}

	if err := db.Cols.Shifts.RemoveId(shift.ID); err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusNoContent)
}

func getShiftHistory(w http.ResponseWriter, r *http.Request) {
	var shifts []db.Shift
	userID := context.Get(r, "userID").(bson.ObjectId)
	if adminUserObj, ok := context.GetOk(r, "admin_user"); ok {
		// comes through admin
		userID = adminUserObj.(db.User).ID
	}

	query := db.M{
		"user_id": userID,
		"status":  "complete",
		"deleted": db.M{"$ne": true},
	}

	if len(r.URL.Query().Get("paid")) > 0 {
		query["paid"] = true
	}

	if err := db.Cols.Shifts.Find(query).Sort("-date").All(&shifts); err != nil {
		panic(err)
	}

	syrup.WriteJSON(w, http.StatusOK, shifts)
}
