package db

import "gopkg.in/mgo.v2/bson"

type Permission struct {
	ID     bson.ObjectId `bson:"_id,omitempty" json:"_id"`
	UserID bson.ObjectId `json:"user_id" bson:"user_id"`

	// superadmin/admin/supervisor
	Type         string `json:"type"`
	Restrictions []struct {
		ID   bson.ObjectId `json:"_id,omitempty"`
		Type string        `json"type"`
	} `json:"restrictions"`
}
