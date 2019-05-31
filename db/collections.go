/*
Package models provides data models and collection mappings
*/
package db

import (
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type M bson.M
type collectionsDeclaration struct {
	Users           *mgo.Collection
	Memberships     *mgo.Collection
	Hashes          *mgo.Collection
	Bikes           *mgo.Collection
	BikeHistory     *mgo.Collection
	BikeMaintenance *mgo.Collection
	Garages         *mgo.Collection
	Messages        *mgo.Collection
	Privileges      *mgo.Collection
	Events          *mgo.Collection
	Shifts          *mgo.Collection
}

var Cols collectionsDeclaration

func Setup() {
	Cols = collectionsDeclaration{
		Users:           DB.C("users"),
		Memberships:     DB.C("memberships"),
		Hashes:          DB.C("hashes"),
		Bikes:           DB.C("bikes"),
		BikeHistory:     DB.C("bikes_history"),
		BikeMaintenance: DB.C("bikes_maintenance"),
		Garages:         DB.C("garages"),
		Messages:        DB.C("messages"),
		Privileges:      DB.C("privileges"),
		Events:          DB.C("events"),
		Shifts:          DB.C("shifts"),
	}
}
