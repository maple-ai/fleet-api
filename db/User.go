package db

import (
	"crypto/rand"
	"errors"
	"io"
	"strings"
	"time"

	"golang.org/x/crypto/scrypt"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"github.com/maple-ai/fleet-api/config"
)

type User struct {
	ID    bson.ObjectId `bson:"_id,omitempty" json:"_id"`
	Email string        `bson:"email" json:"email"`
	Name  string        `bson:"name" json:"name"`

	Password   []byte    `bson:"password,omitempty" json:"-"`
	Salt       []byte    `bson:"salt,omitempty" json:"-"`
	Created    time.Time `bson:"created" json:"created"`
	LastLogin  time.Time `bson:"last_login" json:"last_login"`
	LastAccess time.Time `bson:"last_access" json:"last_access"`

	StripeUserID      string `bson:"stripe_user_id" json:"stripe_user_id"`
	NoUnspentCriminal bool   `bson:"no_unspent_criminal" json:"no_unspent_criminal"`

	Blocked   bool `json:"blocked"`
	Protected bool `json:"protected"`
}

// Not used currently, was thought to be used
type UserActivity struct {
	UserID bson.ObjectId `bson:"user_id" json:"user_id"`

	// create/delete/update
	ActivityType string `bson:"activity_type" json:"activity_type"`

	// name of activity
	Name string `json:"name"`

	// Link to said object (if any)
	ObjectRef  bson.ObjectId `bson:"object_ref,omitempty" json:"object_ref"`
	ObjectType string        `bson:"object_type" json:"object_type"`
}

type UserMembership struct {
	MID bson.ObjectId `bson:"_id,omitempty" json:"_id"`
	// ID is empty until approved
	ID     int           `json:"id"`
	UserID bson.ObjectId `json:"user_id" bson:"user_id"`

	DoB         time.Time `json:"dob"`
	PhoneNumber string    `json:"phone_number" bson:"phone_number"`
	PaypalEmail string    `bson:"paypal_email" json:"paypal_email"`

	Address  string `json:"address"`
	City     string `json:"city"`
	Postcode string `json:"postcode"`
	WorkCity string `json:"work_city" bson:"work_city"`

	Nationality           string    `json:"nationality"`
	DriverLicense         string    `json:"driver_license" bson:"driver_license"`
	DriverLicenseExpiry   time.Time `json:"driver_license_expiry" bson:"driver_license_expiry,omitempty"`
	CbtExpiry             time.Time `json:"cbt_expiry" bson:"cbt_expiry,omitempty"`
	WorkDeclaration       string    `json:"work_declaration" bson:"work_declaration"`
	NoCriminalRecord      bool      `json:"criminal_record" bson:"criminal_record"`
	CriminalRecordDetails string    `json:"criminal_record_details" bson:"criminal_record_details"`
	License               string    `json:"license"`
	UseOwnBike            bool      `json:"use_own_bike" bson:"use_own_bike"`
	NextOfKin             string    `json:"next_of_kin" bson:"next_of_kin"`

	CheckCode         string `json:"check_code" bson:"check_code"`
	UTR               string `json:"utr" bson:"utr"`
	NationalInsurance string `json:"national_insurance" bson:"national_insurance"`

	Submitted     bool          `json:"submitted"`
	SubmittedDate time.Time     `json:"submitted_date" bson:"submitted_date"`
	Approved      bool          `json:"approved"`
	ApprovedDate  time.Time     `json:"approved_date" bson:"approved_date"`
	ApprovedBy    bson.ObjectId `json:"approved_by" bson:"approved_by,omitempty"`

	InterviewDate *time.Time `json:"interview_date" bson:"interview_date,omitempty"`

	Insurance      bool   `json:"insurance" bson:"insurance"`
	InsuranceAdded string `json:"insurance_added" bson:"insurance_added"`

	PrivateNotes string `json:"private_notes" bson:"private_notes"`
	Rating       int    `json:"rating" bson:"rating"`
}

func FindUserByID(ID bson.ObjectId) (*User, error) {
	var user User

	if !ID.Valid() {
		return nil, errors.New("Invalid ID")
	}

	err := Cols.Users.FindId(ID).Select(M{"notes": 0}).One(&user)
	if err != nil && err != mgo.ErrNotFound {
		return nil, err
	}
	if err == mgo.ErrNotFound {
		return nil, nil
	}

	return &user, nil
}

type Hash struct {
	Hash []byte `bson:"hash"`
	Salt []byte `bson:"salt"`
}

func HashPassword(password string) ([]byte, []byte) {
	salt1 := make([]byte, 32)
	salt2 := make([]byte, 32)

	if _, err := io.ReadFull(rand.Reader, salt1); err != nil {
		panic(err)
	}
	if _, err := io.ReadFull(rand.Reader, salt2); err != nil {
		panic(err)
	}

	// hash1
	hash, err := scrypt.Key([]byte(password), salt1, config.ScryptWorkFactor, config.ScryptBlockSize, config.ScryptParallization, 32)
	if err != nil {
		panic(err)
	}

	if err := Cols.Hashes.Insert(M{
		"hash": hash,
		"salt": salt2,
	}); err != nil {
		panic(err)
	}

	// hash2
	hash, err = scrypt.Key([]byte(password), salt2, config.ScryptWorkFactor, config.ScryptBlockSize, config.ScryptParallization, 32)
	if err != nil {
		panic(err)
	}

	return hash, salt1
}

// IsPasswordSecure determines whether password is secure or not
func IsPasswordSecure(password string) bool {
	return !(password == strings.ToLower(password) || len(password) < 8)
}

func (user *User) UpdatePassword(password string) error {
	if IsPasswordSecure(password) == false {
		return errors.New("Weak Password (must have at least 1 capital letter and over 8 characters)")
	}

	hash, salt := HashPassword(password)

	if err := Cols.Users.UpdateId(user.ID, M{
		"$set": M{
			"password": hash,
			"salt":     salt,
		},
	}); err != nil {
		return err
	}

	return nil
}

func (user *User) GetName() string {
	return strings.Split(user.Name, " ")[0]
}
