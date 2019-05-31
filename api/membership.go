package api

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/context"
	"github.com/gorilla/mux"
	"github.com/maple-ai/fleet-api/config"
	"github.com/maple-ai/fleet-api/db"
	"github.com/maple-ai/syrup"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

func userMembershipMiddleware(w http.ResponseWriter, r *http.Request) {
	var membership db.UserMembership

	if err := db.Cols.Memberships.Find(db.M{
		"user_id": context.Get(r, "userID").(bson.ObjectId),
	}).Select(db.M{"private_notes": 0}).One(&membership); err != nil && err != mgo.ErrNotFound {
		panic(err)
	} else if err == nil {
		context.Set(r, "membership", membership)
	}
}

func userMustMembershipMiddleware(w http.ResponseWriter, r *http.Request) {
	if _, ok := context.GetOk(r, "membership"); !ok {
		w.WriteHeader(http.StatusForbidden)
		return
	}
}

func getUserMembership(w http.ResponseWriter, r *http.Request) {
	var membership db.UserMembership
	if membershipObj, ok := context.GetOk(r, "membership"); ok {
		membership = membershipObj.(db.UserMembership)
	}

	syrup.WriteJSON(w, http.StatusOK, membership)
}

func updateUserMembershipDetails(w http.ResponseWriter, r *http.Request) {
	var membership struct {
		DoB         time.Time `json:"dob"`
		PhoneNumber string    `json:"phone_number" bson:"phone_number"`
		PaypalEmail string    `bson:"paypal_email" json:"paypal_email"`

		Address  string `json:"address"`
		City     string `json:"city"`
		Postcode string `json:"postcode"`
		WorkCity string `json:"work_city"`

		Nationality     string `json:"nationality"`
		DriverLicense   string `json:"driver_license" bson:"driver_license"`
		WorkDeclaration string `json:"work_declaration" bson:"work_declaration"`
		// CriminalRecord        bool      `json:"criminal_record" bson:"criminal_record"`
		// CriminalRecordDetails string    `json:"criminal_record_details" bson:"criminal_record_details"`
		License             string    `json:"license"`
		UseOwnBike          bool      `json:"use_own_bike" bson:"use_own_bike"`
		NextOfKin           string    `json:"next_of_kin" bson:"next_of_kin"`
		DriverLicenseExpiry time.Time `json:"driver_license_expiry" bson:"driver_license_expiry,omitempty"`
		CbtExpiry           time.Time `json:"cbt_expiry" bson:"cbt_expiry,omitempty"`

		CheckCode         string `json:"check_code" bson:"check_code"`
		UTR               string `json:"utr" bson:"utr"`
		NationalInsurance string `json:"national_insurance" bson:"national_insurance"`
	}
	if err := syrup.Bind(w, r, &membership); err != nil {
		return
	}

	m := db.UserMembership{
		UserID:          context.Get(r, "userID").(bson.ObjectId),
		DoB:             membership.DoB,
		PhoneNumber:     membership.PhoneNumber,
		PaypalEmail:     membership.PaypalEmail,
		Address:         membership.Address,
		City:            membership.City,
		WorkCity:        membership.WorkCity,
		Postcode:        membership.Postcode,
		Nationality:     membership.Nationality,
		DriverLicense:   membership.DriverLicense,
		WorkDeclaration: membership.WorkDeclaration,
		// NoCriminalRecord:      membership.CriminalRecord,
		// CriminalRecordDetails: membership.CriminalRecordDetails,
		License:             membership.License,
		UseOwnBike:          membership.UseOwnBike,
		CheckCode:           membership.CheckCode,
		UTR:                 membership.UTR,
		NationalInsurance:   membership.NationalInsurance,
		DriverLicenseExpiry: membership.DriverLicenseExpiry,
		CbtExpiry:           membership.CbtExpiry,
		NextOfKin:           membership.NextOfKin,
	}

	if existingMembershipObj, ok := context.GetOk(r, "membership"); ok {
		existingMembership := existingMembershipObj.(db.UserMembership)

		if existingMembership.Submitted == true {
			// already submitted
			syrup.WriteJSON(w, http.StatusBadRequest, map[string]string{
				"error": "Cannot change submitted application",
			})
			return
		}

		m.MID = existingMembership.MID
		if err := db.Cols.Memberships.UpdateId(existingMembership.MID, m); err != nil {
			panic(err)
		}
	} else {
		if err := db.Cols.Memberships.Insert(&m); err != nil {
			panic(err)
		}
	}

	syrup.WriteJSON(w, http.StatusOK, m)
}

func submitMembership(w http.ResponseWriter, r *http.Request) {
	errs := []string{}
	user, err := db.FindUserByID(context.Get(r, "userID").(bson.ObjectId))
	if err != nil {
		panic(err)
	}

	// hasCard := false
	// cards := card.List(&stripe.CardListParams{Customer: user.StripeUserID})
	// // there's a better way with Meta but for some reason Total is always 0
	// for cards.Next() {
	// 	cards.Card()
	// 	hasCard = true
	// 	break
	// }
	//
	// if !hasCard {
	// 	errs = append(errs, "Payment card not added")
	// }

	// Verify membership details
	var m db.UserMembership
	if membershipObj, ok := context.GetOk(r, "membership"); ok {
		m = membershipObj.(db.UserMembership)
	} else {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	switch {
	case m.DoB.IsZero():
		errs = append(errs, "DoB invalid")
	case m.DoB.After(time.Now().AddDate(-21, 0, 0)):
		// driver under 21 years old
		errs = append(errs, "You must be over 21 years old")
	case len(m.PhoneNumber) == 0:
		errs = append(errs, "Phone Number invalid")
	case len(m.Address) == 0:
		errs = append(errs, "Address invalid")
	case len(m.City) == 0:
		errs = append(errs, "City invalid")
	case len(m.Postcode) == 0:
		errs = append(errs, "Postcode invalid")
	case len(m.Nationality) == 0:
		errs = append(errs, "Nationality invalid")
	case len(m.DriverLicense) == 0:
		errs = append(errs, "DriverLicence invalid")
	// case m.NoCriminalRecord == false && len(m.CriminalRecordDetails) == 0:
	// 	errs = append(errs, "CriminalRecord details not specified")
	case len(m.License) == 0:
		errs = append(errs, "Licences invalid")
	case len(m.CheckCode) == 0:
		errs = append(errs, "Check Code invalid")
	case len(m.NationalInsurance) == 0:
		errs = append(errs, "Nat Insurance code Invalid")
	}

	switch m.WorkDeclaration {
	case "eu_passport", "visa", "indefinite_leave":
		break
	default:
		errs = append(errs, "\"Right to work in the United Kingdom\" not selected")
	}

	requiredLicenseRegex := "-(front|back|selfie|utility)"
	numLicenses := 4
	switch m.License {
	case "full":
		break
	case "cbt":
		requiredLicenseRegex = "-(front|back|cbt|selfie|utility)"
		numLicenses = 5
	default:
		errs = append(errs, "Licence type '"+m.License+"' invalid")
	}

	// Check if has license picture uploaded
	if count, err := db.DB.GridFS("driving_licenses").Find(db.M{
		"filename": db.M{
			"$regex": user.ID.Hex() + requiredLicenseRegex,
		},
	}).Count(); err != nil {
		panic(err)
	} else if count != numLicenses {
		fmt.Printf("%v %v\n", count, numLicenses)
		errs = append(errs, "Required documents were not uploaded")
	}

	if len(errs) > 0 {
		syrup.WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
			"errors": errs,
		})
		return
	}

	m.SubmittedDate = time.Now()
	m.Submitted = true
	if err := db.Cols.Memberships.UpdateId(m.MID, m); err != nil {
		panic(err)
	}

	if message, err := db.NewMail(user.Email, db.MembershipSubmissionSubject, db.MembershipSubmission, map[string]interface{}{
		"UserName": user.GetName(),
	}); err != nil {
		panic(err)
	} else if _, _, err := config.Mail.Send(message); err != nil {
		panic(err)
	}

	syrup.WriteJSON(w, http.StatusCreated, m)

	go func() {
		if config.Config.Live == false {
			return
		}

		db.NewMail("info@maple.ai", "Membership Submitted", `
{{ .UserName }} has submitted the application at {{ .DateTime }}

[maple.ai/admin/memberships](https://maple.ai/admin/memberships)
		`, map[string]interface{}{
			"UserName": user.GetName(),
			"DateTime": time.Now().Format("02/01/2006 15:04"),
		})
	}()
}

func getUserDrivingLicenseInfo(w http.ResponseWriter, r *http.Request) {
	userID := context.Get(r, "userID").(bson.ObjectId)

	var files []struct {
		Filename string `bson:"filename"`
	}
	if err := db.DB.GridFS("driving_licenses").Find(db.M{
		"filename": db.M{
			"$regex": userID.Hex() + "-(front|back|cbt|selfie|passport|utility)",
		},
	}).All(&files); err != nil {
		panic(err)
	}

	response := map[string]bool{
		"front":    false,
		"back":     false,
		"cbt":      false,
		"selfie":   false,
		"passport": false,
		"utility":  false,
	}

	for _, file := range files {
		switch {
		case strings.Contains(file.Filename, "front"):
			response["front"] = true
		case strings.Contains(file.Filename, "back"):
			response["back"] = true
		case strings.Contains(file.Filename, "cbt"):
			response["cbt"] = true
		case strings.Contains(file.Filename, "selfie"):
			response["selfie"] = true
		case strings.Contains(file.Filename, "passport"):
			response["passport"] = true
		case strings.Contains(file.Filename, "utility"):
			response["utility"] = true
		}
	}

	syrup.WriteJSON(w, http.StatusOK, response)
}

func getUserDrivingLicensePicture(w http.ResponseWriter, r *http.Request) {
	userID := context.Get(r, "userID").(bson.ObjectId).Hex()

	switch mux.Vars(r)["license_type"] {
	case "front", "back", "cbt", "selfie", "passport", "utility":
		break
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	file, err := db.DB.GridFS("driving_licenses").Open(userID + "-" + mux.Vars(r)["license_type"])
	if err != nil && err != mgo.ErrNotFound {
		panic(err)
	} else if err == mgo.ErrNotFound {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
		return
	}

	io.Copy(w, file)
}

func uploadUserDrivingLicense(w http.ResponseWriter, r *http.Request) {
	userID := context.Get(r, "userID").(bson.ObjectId)
	if adminUserObj, ok := context.GetOk(r, "admin_user"); ok {
		// comes through admin
		userID = adminUserObj.(db.User).ID
	}

	r.ParseMultipartForm(51200)
	f := r.MultipartForm.File["file"][0]
	file, _ := f.Open()

	switch mux.Vars(r)["license_type"] {
	case "front", "back", "cbt", "selfie", "passport", "utility":
		break
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	fileID := userID.Hex() + "-" + mux.Vars(r)["license_type"]

	db.DB.GridFS("driving_licenses").Remove(fileID)
	gridFile, err := db.DB.GridFS("driving_licenses").Create(fileID)
	if err != nil {
		panic(err)
	}

	reader := bufio.NewReader(file)
	defer file.Close()
	defer gridFile.Close()

	buf := make([]byte, 1024)
	for {
		n, err := reader.Read(buf)
		if err != nil && err != io.EOF {
			panic(err)
		}

		if n == 0 || err == io.EOF {
			// EOF
			break
		}

		if _, err := gridFile.Write(buf[:n]); err != nil {
			panic(err)
		}
	}

	w.WriteHeader(204)
}

func deleteUserDrivingLicense(w http.ResponseWriter, r *http.Request) {
	userID := context.Get(r, "userID").(bson.ObjectId)
	if adminUserObj, ok := context.GetOk(r, "admin_user"); ok {
		// comes through admin
		userID = adminUserObj.(db.User).ID
	}

	switch mux.Vars(r)["license_type"] {
	case "front", "back", "cbt", "selfie", "passport", "utility":
		break
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := db.DB.GridFS("driving_licenses").Remove(userID.Hex() + "-" + mux.Vars(r)["license_type"]); err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusNoContent)
}
