package api

import (
	"bufio"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/maple-ai/syrup"
	"github.com/gorilla/context"
	"github.com/gorilla/mux"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"github.com/maple-ai/fleet-api/db"
)

func adminGetBike(w http.ResponseWriter, r *http.Request) {
	syrup.WriteJSON(w, http.StatusOK, context.Get(r, "bike"))
}

func adminGetBikes(w http.ResponseWriter, r *http.Request) {
	var bikes []db.Bike

	q := db.M{"archived": false}
	if available := r.URL.Query().Get("available"); len(available) > 0 {
		q["available"] = true
	}

	if err := db.Cols.Bikes.Find(q).Sort("bike_number").All(&bikes); err != nil {
		panic(err)
	}

	syrup.WriteJSON(w, http.StatusOK, bikes)
}

func adminGetBikesNeedMaintenance(w http.ResponseWriter, r *http.Request) {
	var bikes []db.M
	if err := db.Cols.BikeMaintenance.Pipe([]db.M{
		{"$match": db.M{"mechanic_required": true}},
		{"$lookup": db.M{
			"from":         "bikes",
			"localField":   "bike_id",
			"foreignField": "_id",
			"as":           "bike",
		}},
		{"$unwind": "$bike"},
		{"$sort": db.M{
			"bike.registration": -1,
		}},
		{"$group": db.M{
			"_id": "$bike_id",
			"bike": db.M{
				"$push": "$bike",
			},
		}},
		{"$unwind": "$bike"},
	}).All(&bikes); err != nil {
		panic(err)
	}

	syrup.WriteJSON(w, http.StatusOK, bikes)
}

func adminGetBikesNeedShiftMaintenance(w http.ResponseWriter, r *http.Request) {
	var bikes []db.M
	if err := db.Cols.BikeHistory.Pipe([]db.M{
		{"$match": db.M{"mechanic_required": true}},
		{"$lookup": db.M{
			"from":         "bikes",
			"localField":   "bike_id",
			"foreignField": "_id",
			"as":           "bike",
		}},
		{"$unwind": "$bike"},
		{"$sort": db.M{
			"bike.registration": -1,
		}},
		{"$group": db.M{
			"_id": "$bike_id",
			"bike": db.M{
				"$push": "$bike",
			},
		}},
		{"$unwind": "$bike"},
	}).All(&bikes); err != nil {
		panic(err)
	}

	syrup.WriteJSON(w, http.StatusOK, bikes)
}

func adminSaveBike(w http.ResponseWriter, r *http.Request) {
	var bike db.Bike
	if err := syrup.Bind(w, r, &bike); err != nil {
		return
	}

	// find garage
	garage, err := db.FindGarageByID(bike.GarageID)
	if err != nil {
		panic(err)
	}

	if garage == nil {
		syrup.WriteJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Garage does not exist",
		})

		return
	}

	if r.Method == "POST" {
		bike.ID = bson.NewObjectId()
		bike.Created = time.Now()
		bike.CreatedBy = context.Get(r, "userID").(bson.ObjectId)

		if err := db.Cols.Bikes.Insert(&bike); err != nil {
			panic(err)
		}

		syrup.WriteJSON(w, http.StatusCreated, bike)
		return
	}

	bikeID := context.Get(r, "bike").(db.Bike).ID
	bike.ID = bikeID
	if err := db.Cols.Bikes.UpdateId(bikeID, db.M{"$set": &bike}); err != nil {
		panic(err)
	}

	syrup.WriteJSON(w, http.StatusOK, bike)
}

func adminDeleteBike(w http.ResponseWriter, r *http.Request) {
	var reason struct {
		Reason string `json:"reason"`
	}
	if err := syrup.Bind(w, r, &reason); err != nil {
		return
	}

	if len(reason.Reason) == 0 {
		syrup.WriteJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Please provide a reason for archiving this bike",
		})
		return
	}

	bike := context.Get(r, "bike").(db.Bike)
	if bike.Archived {
		syrup.WriteJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Bike is already archived",
		})
		return
	}

	if err := db.Cols.Bikes.UpdateId(bike.ID, db.M{
		"$set": db.M{
			"archived":        true,
			"archived_reason": reason.Reason,
			"archived_by":     context.Get(r, "userID").(bson.ObjectId),
			"available":       false,
		},
	}); err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusNoContent)
}

func adminGetBikeOperatorNotes(w http.ResponseWriter, r *http.Request) {
	bike := context.Get(r, "bike").(db.Bike)

	var notes []db.M
	if err := db.Cols.BikeHistory.Pipe([]db.M{
		{"$match": db.M{"bike_id": bike.ID}},
		{"$lookup": db.M{
			"from":         "bikes",
			"localField":   "bike_id",
			"foreignField": "_id",
			"as":           "bike",
		}},
		{"$lookup": db.M{
			"from":         "shifts",
			"localField":   "shift_id",
			"foreignField": "_id",
			"as":           "shift",
		}},
		{"$lookup": db.M{
			"from":         "users",
			"localField":   "checked_by",
			"foreignField": "_id",
			"as":           "checkedBy",
		}},
		{"$sort": db.M{
			"shift.date": -1,
		}},
	}).All(&notes); err != nil {
		panic(err)
	}

	syrup.WriteJSON(w, http.StatusOK, notes)
}

func adminGetShiftOperatorNotes(w http.ResponseWriter, r *http.Request) {
	var shift db.Shift
	if err := db.Cols.Shifts.FindId(bson.ObjectIdHex(mux.Vars(r)["shift_id"])).One(&shift); err != nil {
		panic(err)
	}

	var notes db.BikeHistory
	if err := db.Cols.BikeHistory.Find(db.M{"shift_id": shift.ID}).One(&notes); err != nil && err != mgo.ErrNotFound {
		panic(err)
	}

	syrup.WriteJSON(w, http.StatusOK, notes)
}

func adminSetShiftOperatorNotes(w http.ResponseWriter, r *http.Request) {
	var shift db.Shift
	if err := db.Cols.Shifts.FindId(bson.ObjectIdHex(mux.Vars(r)["shift_id"])).One(&shift); err != nil {
		panic(err)
	}

	var body db.BikeHistory
	if err := syrup.Bind(w, r, &body); err != nil {
		return
	}

	body.ShiftID = shift.ID
	body.BikeID = shift.ScooterID
	// body.ID = bson.NewObjectId()
	body.CheckedBy = context.Get(r, "userID").(bson.ObjectId)

	// if body.MechanicRequired alert mechanic? by email

	if _, err := db.Cols.BikeHistory.Upsert(db.M{
		"shift_id": shift.ID,
	}, body); err != nil {
		panic(err)
	}

	syrup.WriteJSON(w, http.StatusCreated, body)
}

func adminGetBikeMaintenance(w http.ResponseWriter, r *http.Request) {
	bike := context.Get(r, "bike").(db.Bike)

	var log []db.BikeMaintenance
	if err := db.Cols.BikeMaintenance.Find(db.M{
		"bike_id": bike.ID,
	}).Sort("-checked_at").All(&log); err != nil {
		panic(err)
	}

	syrup.WriteJSON(w, http.StatusOK, log)
}

func adminSetBikeMaintenance(w http.ResponseWriter, r *http.Request) {
	var log db.BikeMaintenance
	if err := syrup.Bind(w, r, &log); err != nil {
		return
	}

	log.BikeID = context.Get(r, "bike").(db.Bike).ID
	log.CheckedBy = context.Get(r, "userID").(bson.ObjectId)

	status := http.StatusCreated
	if r.Method == "PUT" {
		status = http.StatusOK
		log.ID = bson.ObjectIdHex(mux.Vars(r)["maintenance_log_id"])
		if err := db.Cols.BikeMaintenance.UpdateId(log.ID, &log); err != nil {
			panic(err)
		}
	} else if err := db.Cols.BikeMaintenance.Insert(&log); err != nil {
		panic(err)
	}

	syrup.WriteJSON(w, status, log)
}

func adminDeleteBikeMaintenance(w http.ResponseWriter, r *http.Request) {
	logID := bson.ObjectIdHex(mux.Vars(r)["maintenance_log_id"])

	if err := db.Cols.BikeMaintenance.RemoveId(logID); err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusNoContent)
}

func adminSetMaintenanceAttachment(w http.ResponseWriter, r *http.Request) {
	logID := bson.ObjectIdHex(mux.Vars(r)["maintenance_log_id"])

	r.ParseMultipartForm(51200)
	f := r.MultipartForm.File["file"][0]
	file, _ := f.Open()

	gridFile, err := db.DB.GridFS("maintenance").Create(logID.Hex())
	if err != nil {
		panic(err)
	}

	contentDisposition := f.Header.Get("content-disposition")
	fileName := "unknown"
	if len(contentDisposition) > 0 {
		split := strings.Split(contentDisposition, "filename=\"")
		if len(split) > 1 {
			fileName = strings.Replace(split[1], "\"", "", -1)
		}
	}
	gridFile.SetContentType(f.Header.Get("content-type"))
	gridFile.SetMeta(map[string]string{
		"name": fileName,
	})

	reader := bufio.NewReader(file)
	defer file.Close()
	defer gridFile.Close()

	buf := make([]byte, 1024)
	for {
		n, err := reader.Read(buf)
		if err != nil && err != io.EOF {
			panic(err)
		}

		if n == 0 || err == io.EOF {
			// EOF
			break
		}

		if _, err := gridFile.Write(buf[:n]); err != nil {
			panic(err)
		}
	}

	w.WriteHeader(204)
}

func adminGetMaintenanceAttachment(w http.ResponseWriter, r *http.Request) {
	logID := bson.ObjectIdHex(mux.Vars(r)["maintenance_log_id"])

	file, err := db.DB.GridFS("maintenance").Open(logID.Hex())
	if err != nil && err != mgo.ErrNotFound {
		panic(err)
	} else if err == mgo.ErrNotFound {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("content-type", file.ContentType())

	var meta map[string]string
	if err := file.GetMeta(&meta); err == nil {
		w.Header().Set("content-disposition", "attachment; filename=\""+meta["name"]+"\"")
	}

	io.Copy(w, file)
}

func adminDeleteMaintenanceAttachment(w http.ResponseWriter, r *http.Request) {
	logID := bson.ObjectIdHex(mux.Vars(r)["maintenance_log_id"])

	if err := db.DB.GridFS("maintenance").Remove(logID.Hex()); err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusNoContent)
}
