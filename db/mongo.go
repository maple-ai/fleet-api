/*
Package db provides functions to connect to Mongo. The future will connect it to redis.
*/
package db

import (
	"gopkg.in/mgo.v2"
	"github.com/maple-ai/fleet-api/config"
)

var DB *mgo.Database

// Connect reads config `mongo_url` and `mongo_db` properties
// After connecting it sets up collections into `Cols`
func Connect() error {
	if DB != nil {
		return nil
	}

	sess, err := mgo.Dial(config.Config.MongoURL)
	if err != nil {
		return err
	}

	// https://godoc.org/gopkg.in/mgo.v2#Mode
	sess.SetMode(mgo.Monotonic, true)
	DB = sess.DB(config.Config.MongoDB)

	return nil
}
