package db

import (
	"time"

	"gopkg.in/mgo.v2/bson"
)

type HelpItem struct {
	ID bson.ObjectId `bson:"_id,omitempty" json:"_id"`

	Title       string `json:"title"`
	Description string `json:"desciption"`

	Published bool      `json:"published"`
	Created   time.Time `json:"created"`

	Modifications []struct {
		Title       string `json:"title"`
		Description string `json:"description"`

		Updated   time.Time     `json:"updated"`
		UpdatedBy bson.ObjectId `json:"updated_by"`
	} `json:"modifications"`
}
