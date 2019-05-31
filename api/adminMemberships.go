package api

import (
	"io"
	"net/http"
	"time"

	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"

	"github.com/gorilla/context"
	"github.com/gorilla/mux"
	"github.com/maple-ai/syrup"
	stripe "github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/card"
	"github.com/maple-ai/fleet-api/config"
	"github.com/maple-ai/fleet-api/db"
)

func adminGetMemberships(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")

	if state == "new" {
		var members []db.M
		db.Cols.Users.Pipe([]db.M{
			db.M{"$lookup": db.M{
				"from":         "memberships",
				"localField":   "_id",
				"foreignField": "user_id",
				"as":           "memberships",
			}},
			db.M{"$match": db.M{
				"$or": []db.M{
					db.M{"memberships": db.M{"$size": 0}},
					db.M{"memberships.submitted": false},
				},
			}},
		}).All(&members)

		syrup.WriteJSON(w, http.StatusOK, members)
		return
	}

	q := db.M{
		"submitted": true,
		"approved":  false,
	}

	if state != "contacted" {
		q["interview_date"] = nil
	} else {
		q["interview_date"] = db.M{
			"$not": db.M{
				"$eq": nil,
			},
		}
	}

	var members []db.UserMembership
	query := db.Cols.Memberships.Find(q)
	if err := query.All(&members); err != nil {
		panic(err)
	}

	userMembers := make([]struct {
		db.User
		Membership db.UserMembership `json:"membership"`
	}, len(members))

	for i := range members {
		userMembers[i].Membership = members[i]

		if err := db.Cols.Users.Find(db.M{
			"_id": members[i].UserID,
		}).Select(db.M{"name": 1, "email": 1, "_id": -1}).One(&userMembers[i].User); err != nil {
			panic(err)
		}
	}

	syrup.WriteJSON(w, http.StatusOK, userMembers)
}

func adminGetMembershipStats(w http.ResponseWriter, r *http.Request) {
	uncontacted, err := db.Cols.Memberships.Find(db.M{
		"submitted":      true,
		"approved":       false,
		"interview_date": nil,
	}).Count()

	if err != nil {
		panic(err)
	}

	contacted, err := db.Cols.Memberships.Find(db.M{
		"submitted": true,
		"approved":  false,
		"interview_date": db.M{
			"$not": db.M{
				"$eq": nil,
			},
		},
	}).Count()

	if err != nil {
		panic(err)
	}

	syrup.WriteJSON(w, http.StatusOK, map[string]int{
		"uncontacted": uncontacted,
		"contacted":   contacted,
	})
}

func adminGetUserMembership(w http.ResponseWriter, r *http.Request) {
	user := context.Get(r, "admin_user").(db.User)

	var membership db.UserMembership
	if err := db.Cols.Memberships.Find(db.M{
		"user_id": user.ID,
	}).One(&membership); err != nil && err != mgo.ErrNotFound {
		panic(err)
	}

	syrup.WriteJSON(w, http.StatusOK, membership)
}

func adminUpdateUserMembership(w http.ResponseWriter, r *http.Request) {
	user := context.Get(r, "admin_user").(db.User)

	var originalMembership db.UserMembership
	var membership db.UserMembership

	// Fetch original membership
	if err := db.Cols.Memberships.Find(db.M{
		"user_id": user.ID,
	}).One(&originalMembership); err != nil && err != mgo.ErrNotFound {
		panic(err)
	} else if err == mgo.ErrNotFound {
		originalMembership = db.UserMembership{
			MID:    bson.NewObjectId(),
			UserID: user.ID,
		}

		db.Cols.Memberships.Insert(&originalMembership)
	}

	// Parse incoming membership form
	if err := syrup.Bind(w, r, &membership); err != nil {
		panic(err)
	}

	// keep IDs
	membership.ID = originalMembership.ID
	membership.MID = originalMembership.MID
	membership.UserID = originalMembership.UserID

	// not to be set here
	membership.Submitted = originalMembership.Submitted
	membership.SubmittedDate = originalMembership.SubmittedDate
	membership.Approved = originalMembership.Approved
	membership.ApprovedDate = originalMembership.ApprovedDate
	membership.ApprovedBy = originalMembership.ApprovedBy
	membership.InterviewDate = originalMembership.InterviewDate

	if err := db.Cols.Memberships.UpdateId(membership.MID, membership); err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusNoContent)
}

func adminGetUserDriverLicense(w http.ResponseWriter, r *http.Request) {
	user := context.Get(r, "admin_user").(db.User)
	file, err := db.DB.GridFS("driving_licenses").Open(user.ID.Hex() + "-" + mux.Vars(r)["license_type"])
	if err != nil && err != mgo.ErrNotFound {
		panic(err)
	} else if err == mgo.ErrNotFound {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
		return
	}

	io.Copy(w, file)
}

func adminGetUserPaymentInformation(w http.ResponseWriter, r *http.Request) {
	user := context.Get(r, "admin_user").(db.User)

	if len(user.StripeUserID) == 0 {
	}
	w.WriteHeader(http.StatusNotFound)
	return

	cards := card.List(&stripe.CardListParams{
		Customer: user.StripeUserID,
	})
	for cards.Next() {
		syrup.WriteJSON(w, http.StatusOK, cards.Card())
		return
	}

	if err := cards.Err(); err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusNoContent)
}

func adminSetInterviewDate(w http.ResponseWriter, r *http.Request) {
	user := context.Get(r, "admin_user").(db.User)
	var body struct {
		InterviewDate time.Time `json:"interview_date"`
	}
	if err := syrup.Bind(w, r, &body); err != nil {
		panic(err)
	}

	body.InterviewDate = body.InterviewDate.In(time.Now().Location())

	if body.InterviewDate.Before(time.Now()) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var membership db.UserMembership
	if err := db.Cols.Memberships.Find(db.M{
		"user_id": user.ID,
	}).One(&membership); err != nil {
		panic(err)
	}

	if err := db.Cols.Memberships.UpdateId(membership.MID, db.M{"$set": db.M{"interview_date": body.InterviewDate}}); err != nil {
		panic(err)
	}

	// send 'interview in x' email
	subject := db.MembershipInterviewSubject

	if membership.InterviewDate != nil {
		// send update email
		subject = db.MembershipInterviewRescheduledSubject
	}

	if message, err := db.NewMail(user.Email, subject, db.MembershipInterview, map[string]interface{}{
		"UserName": user.GetName(),
		"Date":     body.InterviewDate.Format("02/01/2006 at 15:04"),
	}); err != nil {
		panic(err)
	} else if _, _, err := config.Mail.Send(message); err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusNoContent)
}

func adminSetRating(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Rating int `json:"rating"`
	}
	if err := syrup.Bind(w, r, &body); err != nil {
		return
	}

	user := context.Get(r, "admin_user").(db.User)
	var membership db.UserMembership
	if err := db.Cols.Memberships.Find(db.M{
		"user_id": user.ID,
	}).One(&membership); err != nil {
		panic(err)
	}

	if err := db.Cols.Memberships.UpdateId(membership.MID, db.M{"$set": db.M{"rating": body.Rating}}); err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusNoContent)
}

func adminAcceptMembership(w http.ResponseWriter, r *http.Request) {
	user := context.Get(r, "admin_user").(db.User)

	var membership db.UserMembership
	if err := db.Cols.Memberships.Find(db.M{
		"user_id": user.ID,
	}).One(&membership); err != nil {
		panic(err)
	}

	if membership.Approved {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// find last ID
	var membershipID db.UserMembership
	if err := db.Cols.Memberships.Find(db.M{
		"approved": true,
	}).Sort("-id").
		Select(db.M{"id": 1, "_id": 0}).
		One(&membershipID); err != nil && err != mgo.ErrNotFound {
		panic(err)
	} else if err == mgo.ErrNotFound {
		membershipID.ID = 100000
	} else {
		membershipID.ID++
	}

	if err := db.Cols.Memberships.UpdateId(membership.MID, db.M{
		"$set": db.M{
			"approved":      true,
			"approved_by":   context.Get(r, "userID").(bson.ObjectId),
			"approved_date": time.Now(),
			"id":            membershipID.ID,
		},
	}); err != nil {
		panic(err)
	}

	if message, err := db.NewMail(user.Email, db.MembershipAcceptedSubject, db.MembershipAccepted, map[string]interface{}{
		"UserName":     user.GetName(),
		"MembershipID": membershipID.ID,
	}); err != nil {
		panic(err)
	} else if _, _, err := config.Mail.Send(message); err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusNoContent)
}

func adminRejectMembership(w http.ResponseWriter, r *http.Request) {
	user := context.Get(r, "admin_user").(db.User)

	var membership db.UserMembership
	if err := db.Cols.Memberships.Find(db.M{
		"user_id": user.ID,
	}).One(&membership); err != nil {
		panic(err)
	}

	if membership.Approved || !membership.Submitted {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// delete membership application
	if err := db.Cols.Memberships.RemoveId(membership.MID); err != nil {
		panic(err)
	}

	if message, err := db.NewMail(user.Email, db.MembershipRejectedSubject, db.MembershipRejected, map[string]interface{}{
		"UserName": user.GetName(),
	}); err != nil {
		panic(err)
	} else if _, _, err := config.Mail.Send(message); err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusNoContent)
}

func adminMoveMembership(w http.ResponseWriter, r *http.Request) {
	user := context.Get(r, "admin_user").(db.User)

	var membership db.UserMembership
	if err := db.Cols.Memberships.Find(db.M{
		"user_id": user.ID,
	}).One(&membership); err != nil {
		panic(err)
	}

	if !membership.Approved || !membership.Submitted {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	db.Cols.Memberships.UpdateId(membership.MID, db.M{"$set": db.M{
		"approved":       false,
		"interview_date": nil,
	}})

	w.WriteHeader(http.StatusNoContent)
}
