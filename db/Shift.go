package db

import (
	"time"

	"gopkg.in/mgo.v2/bson"
)

type Shift struct {
	ID        bson.ObjectId `json:"_id" bson:"_id,omitempty"`
	GarageID  bson.ObjectId `json:"garage_id" bson:"garage_id"`
	ScooterID bson.ObjectId `json:"scooter_id" bson:"scooter_id"`
	UserID    bson.ObjectId `json:"user_id" bson:"user_id"`

	Date     time.Time     `json:"date" bson:"date"`
	Duration time.Duration `json:"duration" bson:"duration"`

	// created/confirmed/cancelled/running/complete
	Status           string        `json:"status" bson:"status"`
	CheckInOperator  bson.ObjectId `json:"check_in_operator" bson:"check_in_operator,omitempty"`
	CheckIn          time.Time     `json:"check_in" bson:"check_in,omitempty"`
	CheckOutOperator bson.ObjectId `json:"check_out_operator" bson:"check_out_operator,omitempty"`
	CheckOut         time.Time     `json:"check_out" bson:"check_out,omitempty"`
	ShiftNotes       string        `json:"shift_notes" bson:"shift_notes"`

	Paid       bool          `json:"paid"`
	PaidAmount float64       `json:"paid_amount" bson:"paid_amount"`
	PaidBy     bson.ObjectId `json:"paid_by" bson:"paid_by,omitempty"`
	PaidAt     time.Time     `json:"paid_at" bson:"paid_at"`

	Added         time.Time     `json:"added" bson:"added"`
	AddedBy       bson.ObjectId `json:"added_by" bson:"added_by"`
	Deleted       bool          `json:"deleted" bson:"deleted"`
	DeletedReason string        `json:"deleted_reason" bson:"deleted_reason"`
	DeletedDate   time.Time     `json:"deleted_date" bson:"deleted_date,omitempty"`
	DeletedBy     bson.ObjectId `json:"deleted_by" bson:"deleted_by,omitempty"`
}

// GetAvailableScooters returns bikes which can be hired
func GetAvailableScooters(garageID bson.ObjectId, date time.Time, maxCC int) []Bike {
	var bikes []Bike
	var availableBikes []Bike

	q := M{
		"garage_id": garageID,
		"available": true,
		"archived":  false,
	}

	if maxCC > 0 {
		q["engine_size"] = M{
			"$lte": maxCC,
		}
	}

	if err := Cols.Bikes.Find(q).Sort("bike_number").All(&bikes); err != nil {
		panic(err)
	}

	start, _ := time.Parse("2006-01-02", date.Format("2006-01-02"))
	end, _ := time.Parse("2006-01-02", date.Add(24*time.Hour).Format("2006-01-02"))

	for _, bike := range bikes {
		var shifts []Shift
		if err := Cols.Shifts.Find(M{
			"garage_id":  garageID,
			"scooter_id": bike.ID,
			"date": M{
				"$gte": start,
				"$lt":  end,
			},
			"deleted": false,
			"status":  "confirmed",
		}).All(&shifts); err != nil {
			panic(err)
		}

		if len(shifts) == 0 {
			availableBikes = append(availableBikes, bike)
		}
	}

	return availableBikes
}
