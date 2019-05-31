package db

import "gopkg.in/mgo.v2/bson"

type Event struct {
	ID bson.ObjectId `bson:"_id,omitempty" json:"_id"`

	BikeID bson.ObjectId `json:"bike_id"`
	Type   string        `json:"type"`
	Level  string        `json:"level"`
}
