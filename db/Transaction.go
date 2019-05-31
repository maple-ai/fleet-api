package db

import "gopkg.in/mgo.v2/bson"

type Transaction struct {
	ID bson.ObjectId `bson:"_id,omitempty" json:"_id"`

	StripeID   string `json:"stripe_id"`
	StripeCard string `json:"stripe_card"`
	StripeUser string `json:"stripe_user"`
	Amount     int64  `json:"amount"`

	Type   string `json:"type"`
	Status string `json:"status"`
}
