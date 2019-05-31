package api

import (
	"net/http"

	"github.com/gorilla/context"
	"github.com/maple-ai/syrup"
	"github.com/maple-ai/fleet-api/db"
)

func adminGetSettings(w http.ResponseWriter, r *http.Request) {
	var settings []db.M

	q := db.M{}
	if _, ok := context.GetOk(r, "is_admin"); !ok {
		q = db.M{
			"name": db.M{
				"$in": []string{"shift_description", "days"},
			},
		}
	}

	db.DB.C("settings").Find(q).All(&settings)

	syrup.WriteJSON(w, http.StatusOK, settings)
}

func adminSetSettings(w http.ResponseWriter, r *http.Request) {
	var body []db.M
	if err := syrup.Bind(w, r, &body); err != nil {
		return
	}

	db.DB.C("settings").RemoveAll(db.M{})

	for _, doc := range body {
		db.DB.C("settings").Insert(doc)
	}

	w.WriteHeader(http.StatusNoContent)
}
