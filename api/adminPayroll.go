package api

import (
	"net/http"
	"time"

	"github.com/gorilla/context"
	"github.com/maple-ai/fleet-api/config"
	"github.com/maple-ai/fleet-api/db"
	"github.com/maple-ai/syrup"
	"gopkg.in/mgo.v2/bson"
)

func adminPayroll(w http.ResponseWriter, r *http.Request) {
	var results []db.M
	if err := db.Cols.Shifts.Pipe([]db.M{
		{"$match": db.M{
			"paid": false,
			// "date":   db.M{"$lte": time.Now()},
			"status": "complete",
		}},
		{"$lookup": db.M{
			"from":         "users",
			"localField":   "user_id",
			"foreignField": "_id",
			"as":           "user",
		}},
		{"$unwind": "$user"},
		{"$lookup": db.M{
			"from":         "memberships",
			"localField":   "user._id",
			"foreignField": "user_id",
			"as":           "membership",
		}},
		{"$unwind": "$membership"},
		{"$sort": db.M{
			"paid": 1,
			"date": 1,
		}},
	}).All(&results); err != nil {
		panic(err)
	}

	syrup.WriteJSON(w, http.StatusOK, results)
}

func adminPayout(w http.ResponseWriter, r *http.Request) {
	var body struct {
		UserID bson.ObjectId `json:"user"`
		Shifts []struct {
			Shift   bson.ObjectId `json:"shift"`
			Total   float32       `json:"total"`
			Removed bool          `json:"removed"`
		} `json:"shifts"`
	}
	if err := syrup.Bind(w, r, &body); err != nil {
		return
	}

	// find user ID
	var user db.User
	if err := db.Cols.Users.FindId(body.UserID).One(&user); err != nil {
		panic(err)
	}

	// update shifts
	for _, shift := range body.Shifts {
		shiftDoc := db.M{
			"paid":        true,
			"paid_amount": shift.Total,
			"paid_by":     context.Get(r, "userID"),
			"paid_at":     time.Now(),
		}

		if shift.Removed {
			shiftDoc["paid_amount"] = 0
			shiftDoc["deleted"] = true
			shiftDoc["deleted_reason"] = "Payroll"
			shiftDoc["deleted_date"] = time.Now()
			shiftDoc["deleted_by"] = context.Get(r, "userID")
		}

		db.Cols.Shifts.UpdateId(shift.Shift, db.M{"$set": shiftDoc})
	}

	// _, err := config.PaypalClient.GetAccessToken()
	// payout := paypal.Payout{
	// 	SenderBatchHeader: &paypal.SenderBatchHeader{
	// 		EmailSubject: "Subject",
	// 	},
	// 	Items: []paypal.PayoutItem{
	// 		{
	// 			RecipientType: "EMAIL",
	// 			Receiver:      "-buyer@gmail.com",
	// 			Amount: &paypal.AmountPayout{
	// 				Value:    "10.00",
	// 				Currency: "GBP",
	// 			},
	// 			Note:         "Hello world",
	// 			SenderItemID: "no id",
	// 		},
	// 	},
	// }

	// payoutResp, err := config.PaypalClient.CreateSinglePayout(payout)
	// fmt.Println(payoutResp, err)

	// send alert to user
	if message, err := db.NewMail(user.Email, db.UserPayoutSubject, db.UserPayout, map[string]interface{}{
		"UserName": user.GetName(),
	}); err != nil {
		panic(err)
	} else if _, _, err := config.Mail.Send(message); err != nil {
		panic(err)
	}
}
