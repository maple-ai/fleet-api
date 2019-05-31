package api

import (
	"bytes"
	"net/http"
	"net/mail"
	"strings"

	"github.com/gorilla/context"
	"github.com/maple-ai/syrup"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/card"
	"github.com/stripe/stripe-go/customer"
	"golang.org/x/crypto/scrypt"
	"gopkg.in/mgo.v2/bson"
	"github.com/maple-ai/fleet-api/config"
	"github.com/maple-ai/fleet-api/db"
)

func getUser(w http.ResponseWriter, r *http.Request) {
	user, err := db.FindUserByID(context.Get(r, "userID").(bson.ObjectId))
	if err != nil {
		panic(err)
	}

	if user == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	syrup.WriteJSON(w, http.StatusOK, user)
}

func getUserPrivileges(w http.ResponseWriter, r *http.Request) {
	var privileges []db.Permission

	if err := db.Cols.Privileges.Find(db.M{
		"user_id": context.Get(r, "userID").(bson.ObjectId),
	}).All(&privileges); err != nil {
		panic(err)
	}

	syrup.WriteJSON(w, http.StatusOK, privileges)
}

func getUserBilling(w http.ResponseWriter, r *http.Request) {
	user, _ := db.FindUserByID(context.Get(r, "userID").(bson.ObjectId))
	hasCard := false

	if len(user.StripeUserID) > 0 {
		cards := card.List(&stripe.CardListParams{
			Customer: user.StripeUserID,
		})
		for cards.Next() {
			cards.Card()
			hasCard = true
			break
		}

		if err := cards.Err(); err != nil {
			panic(err)
		}
	}

	syrup.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"user_id":    user.StripeUserID,
		"stripe_pub": config.Config.Stripe.Pub,
		"cards":      hasCard,
	})
}

func addUserCard(w http.ResponseWriter, r *http.Request) {
	user, _ := db.FindUserByID(context.Get(r, "userID").(bson.ObjectId))
	var body struct {
		Token string `json:"token"`
	}
	if err := syrup.Bind(w, r, &body); err != nil {
		return
	}

	if len(user.StripeUserID) > 0 {
		if _, err := card.New(&stripe.CardParams{
			Token:    body.Token,
			Customer: user.StripeUserID,
		}); err != nil {
			panic(err)
		}
	} else {
		params := stripe.CustomerParams{}
		params.SetSource(body.Token)
		cust, err := customer.New(&params)
		if err != nil {
			panic(err)
		}

		if err := db.Cols.Users.UpdateId(user.ID, db.M{"$set": db.M{
			"stripe_user_id": cust.ID,
		}}); err != nil {
			panic(err)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func updateUserPassword(w http.ResponseWriter, r *http.Request) {
	user, _ := db.FindUserByID(context.Get(r, "userID").(bson.ObjectId))
	var pwd struct {
		Password    string `json:"password"`
		NewPassword string `json:"new_password"`
	}
	if err := syrup.Bind(w, r, &pwd); err != nil {
		return
	}

	if len(user.Salt) > 0 {
		// verify current password
		pwdHash, err := scrypt.Key([]byte(pwd.Password), user.Salt, config.ScryptWorkFactor, config.ScryptBlockSize, config.ScryptParallization, 32)
		if err != nil {
			panic(err)
		}

		var hash db.Hash
		err = db.Cols.Hashes.Find(db.M{
			"hash": pwdHash,
		}).One(&hash)
		if err != nil {
			syrup.WriteJSON(w, http.StatusBadRequest, map[string]string{
				"error": "Current Password Incorrect",
			})
			return
		}

		pwdHash, err = scrypt.Key([]byte(pwd.Password), hash.Salt, config.ScryptWorkFactor, config.ScryptBlockSize, config.ScryptParallization, 32)
		if err != nil {
			panic(err)
		}

		// compare
		if !bytes.Equal(user.Password, pwdHash) {
			syrup.WriteJSON(w, http.StatusBadRequest, map[string]string{
				"error": "Current Password Incorrect",
			})
			return
		}
	}

	if err := user.UpdatePassword(pwd.NewPassword); err != nil {
		if strings.Contains(err.Error(), "Weak Password") {
			syrup.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		panic(err)
	}

	w.WriteHeader(http.StatusNoContent)
}

func updateUserProfile(w http.ResponseWriter, r *http.Request) {
	errs := []string{}
	userID := context.Get(r, "userID").(bson.ObjectId)

	var body struct {
		Address     string `json:"address"`
		City        string `json:"city"`
		Postcode    string `json:"postcode"`
		PhoneNumber string `json:"phone_number"`

		Name        string `json:"name"`
		Email       string `json:"email"`
		PaypalEmail string `json:"paypal_email"`
	}
	if err := syrup.Bind(w, r, &body); err != nil {
		return
	}

	switch {
	case len(body.Name) == 0:
		errs = append(errs, "Name cannot be empty")
	case len(body.Email) == 0:
		errs = append(errs, "Email cannot be empty")
	}

	if len(body.Email) > 0 {
		address, err := mail.ParseAddress(strings.ToLower(body.Email))
		body.Email = address.Address

		if err != nil {
			errs = append(errs, "Email address invalid")
		} else if exists, err := db.Cols.Users.Find(db.M{
			"email": address.Address,
			"_id":   db.M{"$not": db.M{"$eq": userID}},
		}).Count(); err != nil {
			panic(err)
		} else if exists > 0 {
			errs = append(errs, "Email address in use")
		}
	}

	if len(body.PaypalEmail) > 0 {
		address, err := mail.ParseAddress(strings.ToLower(body.PaypalEmail))
		body.PaypalEmail = address.Address

		if err != nil {
			errs = append(errs, "Paypal Email address invalid")
		}
	}

	if membershipObj, ok := context.GetOk(r, "membership"); ok {
		membership := membershipObj.(db.UserMembership)

		// update membership
		switch {
		case len(body.Address) == 0:
			errs = append(errs, "Address cannot be empty")
		case len(body.City) == 0:
			errs = append(errs, "City cannot be empty")
		case len(body.Postcode) == 0:
			errs = append(errs, "Postcode cannot be empty")
		}

		if len(errs) == 0 {
			// update membership
			if err := db.Cols.Memberships.UpdateId(membership.MID, db.M{
				"$set": db.M{
					"city":     body.City,
					"address":  body.Address,
					"postcode": body.Postcode,
				},
			}); err != nil {
				panic(err)
			}
		}
	}

	if len(errs) > 0 {
		syrup.WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
			"errors": errs,
		})
		return
	}

	// update user profile
	db.Cols.Users.UpdateId(userID, db.M{
		"$set": db.M{
			"name":         body.Name,
			"email":        body.Email,
			"paypal_email": body.PaypalEmail,
		},
	})
}
