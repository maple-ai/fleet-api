package db

import (
	"time"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type Bike struct {
	ID       bson.ObjectId `bson:"_id,omitempty" json:"_id"`
	GarageID bson.ObjectId `bson:"garage_id" json:"garage_id"`

	Registration     string `json:"registration"`
	BikeNumber       int    `bson:"bike_number" json:"bike_number"`
	VIN              string `json:"vin"`
	RegistrationDate string `json:"registration_date" bson:"registration_date"`
	Price            int64  `json:"price"`
	Available        bool   `json:"available"`
	EngineSize       int    `bson:"engine_size" json:"engine_size"`

	Created   time.Time     `json:"created"`
	CreatedBy bson.ObjectId `json:"created_by" bson:"created_by,omitempty"`

	TrackerID   int    `json:"tracker_id" bson:"tracker_id"`
	PhoneNumber string `json:"phone_number" bson:"phone_number"`
	DeviceID    string `json:"device_id" bson:"device_id"`

	Archived       bool          `json:"archived"`
	ArchivedReason string        `json:"archived_reason" bson:"archived_reason"`
	ArchivedBy     bson.ObjectId `json:"archived_by" bson:"archived_by,omitempty"`
}

type BikeHistory struct {
	ID        bson.ObjectId `bson:"_id,omitempty" json:"_id"`
	BikeID    bson.ObjectId `bson:"bike_id" json:"bike_id"`
	ShiftID   bson.ObjectId `bson:"shift_id" json:"shift_id"`
	CheckedBy bson.ObjectId `bson:"checked_by" json:"checked_by"`
	CheckedAt time.Time     `bson:"checked_at" json:"checked_at"`

	Condition string `json:"condition"`
	Notes     string `json:"notes"`
	FuelLevel int    `bson:"fuel_level" json:"fuel_level"`

	LockedUp        bool `json:"locked_up" bson:"locked_up"`
	ClothesReturned bool `json:"clothes_returned" bson:"clothes_returned"`
	KeyReturned     bool `json:"key_returned" bson:"key_returned"`

	MechanicRequired    bool   `json:"mechanic_required" bson:"mechanic_required"`
	MechanicAlertReason string `bson:"mechanic_alert_reason" json:"mechanic_alert_reason"`
}

type BikeMaintenance struct {
	ID            bson.ObjectId `bson:"_id,omitempty" json:"_id"`
	BikeID        bson.ObjectId `bson:"bike_id" json:"bike_id"`
	Notes         string        `json:"notes"`
	HasAttachment bool          `json:"has_attachment" bson:"has_attachment"`

	CheckedBy bson.ObjectId `bson:"checked_by" json:"checked_by"`
	CheckedAt time.Time     `bson:"checked_at" json:"checked_at"`

	MechanicRequired bool `json:"mechanic_required" bson:"mechanic_required"`
}

type Garage struct {
	ID bson.ObjectId `bson:"_id,omitempty" json:"_id"`

	Name     string `json:"name"`
	Location struct {
		Lat float64 `json:"lat"`
		Lng float64 `json:"lng"`
	} `json:"location"`
	Capacity int `json:"capacity"`
}

func FindGarageByID(ID bson.ObjectId) (*Garage, error) {
	var garage Garage

	if !ID.Valid() {
		return nil, nil
	}

	err := Cols.Garages.FindId(ID).One(&garage)
	if err != nil && err != mgo.ErrNotFound {
		return nil, err
	}

	if err == mgo.ErrNotFound {
		return nil, nil
	}

	return &garage, nil
}
