package api

import (
	"net/http"
	"net/mail"
	"strconv"
	"strings"

	"github.com/gorilla/context"
	"github.com/maple-ai/syrup"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"github.com/maple-ai/fleet-api/config"
	"github.com/maple-ai/fleet-api/db"
)

func adminGetUsers(w http.ResponseWriter, r *http.Request) {
	pipe := []db.M{}
	sort := db.M{
		"membership.approved_date": -1,
	}
	q := r.URL.Query()

	if name := q.Get("name"); len(name) > 0 {
		pipe = append(pipe, db.M{"$match": db.M{"name": db.M{"$regex": ".*" + name + ".*", "$options": "i"}}})
		sort = db.M{
			"name": 1,
		}
	}
	if email := q.Get("email"); len(email) > 0 {
		pipe = append(pipe, db.M{"$match": db.M{"email": db.M{"$regex": ".*" + email + ".*"}}})
		sort = db.M{
			"name": 1,
		}
	}

	if sortQuery := q.Get("sort"); len(sortQuery) > 0 {
		sort = db.M{
			"name": 1,
		}
	}

	pipe = append(pipe,
		db.M{"$lookup": db.M{
			"from":         "memberships",
			"localField":   "_id",
			"foreignField": "user_id",
			"as":           "membership",
		}},
		db.M{"$unwind": "$membership"},
		db.M{"$match": db.M{"membership": db.M{"$ne": nil}}},
		db.M{"$match": db.M{"membership.approved": true}},
		db.M{"$sort": sort},
	)

	if membershipID := q.Get("membership"); len(membershipID) > 0 {
		if membershipInt, err := strconv.Atoi(membershipID); err == nil {
			pipe = append(pipe, db.M{
				"$match": db.M{"membership.id": membershipInt},
			})
		}
	}

	if driverLicense := q.Get("driver_license"); len(driverLicense) > 0 {
		pipe = append(pipe, db.M{
			"$match": db.M{"membership.driver_license": db.M{"$regex": ".*" + driverLicense + ".*"}},
		})
	}

	var results []db.M
	if err := db.Cols.Users.Pipe(pipe).All(&results); err != nil {
		panic(err)
	}

	syrup.WriteJSON(w, http.StatusOK, &results)
}

func adminGetAdminUsers(w http.ResponseWriter, r *http.Request) {
	var results []db.M
	if err := db.Cols.Privileges.Pipe([]db.M{
		{"$lookup": db.M{
			"from":         "users",
			"localField":   "user_id",
			"foreignField": "_id",
			"as":           "user",
		}},
		{"$sort": db.M{
			"user.name": 1,
		}},
	}).All(&results); err != nil {
		panic(err)
	}

	syrup.WriteJSON(w, http.StatusOK, &results)
}

func adminGetUser(w http.ResponseWriter, r *http.Request) {
	user := context.Get(r, "admin_user").(db.User)

	withPermissions := struct {
		User        db.User         `json:"user"`
		Permissions []db.Permission `json:"permissions"`
	}{User: user}

	if err := db.Cols.Privileges.Find(db.M{"user_id": user.ID}).All(&withPermissions.Permissions); err != nil {
		panic(err)
	}

	syrup.WriteJSON(w, http.StatusOK, withPermissions)
}

func adminUserSetPrivileges(w http.ResponseWriter, r *http.Request) {
	user := context.Get(r, "admin_user").(db.User)
	var body struct {
		Type string `json:"type"`
	}
	if err := syrup.Bind(w, r, &body); err != nil {
		return
	}

	if user.Protected {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if _, err := db.Cols.Privileges.RemoveAll(db.M{"user_id": user.ID}); err != nil {
		panic(err)
	}

	switch body.Type {
	case "supervisor", "admin", "superadmin", "mechanic":
		break
	default:
		w.WriteHeader(http.StatusNoContent)
		return
	}

	newPrivilege := db.Permission{
		UserID: user.ID,
		Type:   body.Type,
	}
	if err := db.Cols.Privileges.Insert(&newPrivilege); err != nil {
		panic(err)
	}

	syrup.WriteJSON(w, http.StatusOK, newPrivilege)
}

func adminSaveUser(w http.ResponseWriter, r *http.Request) {
	var user db.User
	if err := syrup.Bind(w, r, &user); err != nil {
		return
	}

	user.ID = context.Get(r, "admin_user").(db.User).ID
	if err := db.Cols.Users.UpdateId(user.ID, db.M{"$set": user}); err != nil {
		panic(err)
	}

	syrup.WriteJSON(w, http.StatusOK, &user)
}

func adminUserSetPassword(w http.ResponseWriter, r *http.Request) {
	var pwd struct {
		Password string `json:"password"`
	}
	if err := syrup.Bind(w, r, &pwd); err != nil {
		return
	}

	user := context.Get(r, "admin_user").(db.User)
	if user.Protected {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := user.UpdatePassword(pwd.Password); err != nil {
		if strings.Contains(err.Error(), "Weak Password") {
			syrup.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		panic(err)
	}

	w.WriteHeader(http.StatusNoContent)
}

func adminSetUserEmail(w http.ResponseWriter, r *http.Request) {
	errs := []string{}
	var body struct {
		Email string `json:"email"`
	}
	if err := syrup.Bind(w, r, &body); err != nil {
		return
	}

	user := context.Get(r, "admin_user").(db.User)
	addr, err := mail.ParseAddress(body.Email)
	if err != nil {
		errs = append(errs, err.Error())
	} else {
		body.Email = strings.ToLower(addr.Address)
		if user.Email == body.Email {
			errs = append(errs, "Email not changed")
		}
	}

	// find dup
	count, _ := db.Cols.Users.Find(db.M{
		"email": body.Email,
	}).Count()
	if count > 0 {
		errs = append(errs, "Email address in use")
	}

	if len(errs) > 0 {
		syrup.WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error": strings.Join(errs, ", "),
		})
		return
	}

	db.Cols.Users.UpdateId(user.ID, db.M{
		"$set": db.M{
			"email": body.Email,
		},
	})

	w.WriteHeader(http.StatusNoContent)
}

func adminSetUserName(w http.ResponseWriter, r *http.Request) {
	errs := []string{}
	var body struct {
		Name string `json:"name"`
	}
	if err := syrup.Bind(w, r, &body); err != nil {
		return
	}

	user := context.Get(r, "admin_user").(db.User)

	if len(body.Name) == 0 {
		errs = append(errs, "Invalid")
	}

	if len(errs) > 0 {
		syrup.WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error": strings.Join(errs, ", "),
		})
		return
	}

	db.Cols.Users.UpdateId(user.ID, db.M{
		"$set": db.M{
			"name": body.Name,
		},
	})

	w.WriteHeader(http.StatusNoContent)
}

func adminDeleteUser(w http.ResponseWriter, r *http.Request) {
	user := context.Get(r, "admin_user").(db.User)

	if user.Protected || user.ID.Hex() == context.Get(r, "userID").(bson.ObjectId).Hex() {
		// cannot delete self
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := db.Cols.Memberships.Remove(db.M{"user_id": user.ID}); err != nil && err != mgo.ErrNotFound {
		panic(err)
	}
	if err := db.Cols.Users.RemoveId(user.ID); err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusNoContent)
}

func blockUser(w http.ResponseWriter, r *http.Request) {
	user := context.Get(r, "admin_user").(db.User)

	if user.Protected || user.ID.Hex() == context.Get(r, "userID").(bson.ObjectId).Hex() {
		// cannot block self
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	blocked := r.Method == "POST"

	if err := db.Cols.Users.UpdateId(user.ID, db.M{"$set": db.M{"blocked": blocked}}); err != nil {
		panic(err)
	}

	subject := db.UserBanSubject
	message := db.UserBan
	if !blocked {
		subject = db.UserUnbanSubject
		message = db.UserUnban
	}

	if message, err := db.NewMail(user.Email, subject, message, map[string]interface{}{}); err != nil {
		panic(err)
	} else if _, _, err := config.Mail.Send(message); err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusNoContent)
}
