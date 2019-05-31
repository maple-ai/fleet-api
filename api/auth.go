package api

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/context"
	"github.com/maple-ai/fleet-api/config"
	"github.com/maple-ai/fleet-api/db"
	"github.com/maple-ai/syrup"
	"golang.org/x/crypto/scrypt"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

func secureMiddleware(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authQuery := r.URL.Query().Get("authorization"); len(authQuery) > 0 {
		authHeader = authQuery
	}

	if len(authHeader) == 0 {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	value := make(map[string]interface{})
	if err := config.Cookie.Decode("Maple Fleet", authHeader, &value); err != nil {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	userID, ok := value["user"]
	if !ok || !bson.IsObjectIdHex(userID.(string)) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	userIDBson := bson.ObjectIdHex(userID.(string))
	context.Set(r, "userID", userIDBson)

	// find if user blocked
	if c, err := db.Cols.Users.Find(db.M{"_id": userIDBson, "blocked": true}).Count(); err != nil {
		panic(err)
	} else if c > 0 {
		// user blocked
		w.WriteHeader(http.StatusForbidden)
		return
	}

	go db.Cols.Users.UpdateId(userIDBson, db.M{
		"$set": db.M{"last_access": time.Now()},
	})
}

func superAdminMiddleware(w http.ResponseWriter, r *http.Request) {
	if count, err := db.Cols.Privileges.Find(db.M{
		"user_id": context.Get(r, "userID").(bson.ObjectId),
		"type":    "superadmin",
	}).Count(); err != nil {
		panic(err)
	} else if count == 0 {
		// no privileges
		w.WriteHeader(http.StatusForbidden)
		return
	}
}

func adminMiddleware(w http.ResponseWriter, r *http.Request) {
	if count, err := db.Cols.Privileges.Find(db.M{
		"user_id": context.Get(r, "userID").(bson.ObjectId),
		"$or": []db.M{
			{"type": "admin"},
			{"type": "superadmin"},
		},
	}).Count(); err != nil {
		panic(err)
	} else if count == 0 {
		// no privileges
		w.WriteHeader(http.StatusForbidden)
		return
	}
}

func supervisorMiddleware(w http.ResponseWriter, r *http.Request) {
	if count, err := db.Cols.Privileges.Find(db.M{
		"user_id": context.Get(r, "userID").(bson.ObjectId),
		"$or": []db.M{
			{"type": "supervisor"},
			{"type": "admin"},
			{"type": "superadmin"},
			{"type": "mechanic"},
		},
	}).Count(); err != nil {
		panic(err)
	} else if count == 0 {
		// no privileges
		w.WriteHeader(http.StatusForbidden)
		return
	}

	context.Set(r, "is_admin", true)
}

func login(w http.ResponseWriter, r *http.Request) {
	var login struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := syrup.Bind(w, r, &login); err != nil {
		return
	}

	if len(login.Email) == 0 || len(login.Password) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var user db.User
	if err := db.Cols.Users.Find(db.M{
		"email":   strings.ToLower(login.Email),
		"blocked": db.M{"$ne": true},
	}).One(&user); err != nil {
		if err == mgo.ErrNotFound {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		panic(err)
	}

	pwdHash, err := scrypt.Key([]byte(login.Password), user.Salt, config.ScryptWorkFactor, config.ScryptBlockSize, config.ScryptParallization, 32)
	if err != nil {
		panic(err)
	}

	var hash db.Hash
	err = db.Cols.Hashes.Find(db.M{
		"hash": pwdHash,
	}).One(&hash)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	pwdHash, err = scrypt.Key([]byte(login.Password), hash.Salt, config.ScryptWorkFactor, config.ScryptBlockSize, config.ScryptParallization, 32)
	if err != nil {
		panic(err)
	}

	// compare
	if !bytes.Equal(user.Password, pwdHash) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := db.Cols.Users.UpdateId(user.ID, db.M{"$set": db.M{"last_login": time.Now()}}); err != nil {
		panic(err)
	}

	encoded, err := config.Cookie.Encode("Maple Fleet", map[string]interface{}{
		"user": user.ID.Hex(),
	})
	if err != nil {
		panic(err)
	}

	syrup.WriteJSON(w, http.StatusOK, map[string]string{
		"token": encoded,
	})
}

func signup(w http.ResponseWriter, r *http.Request) {
	var login struct {
		Name              string `json:"name"`
		Email             string `json:"email"`
		Password          string `json:"password"`
		NoUnspentCriminal bool   `json:"no_unspent_criminal"`
		PhoneNumber       string `json:"phone_number"`
	}
	if err := syrup.Bind(w, r, &login); err != nil {
		return
	}

	errs := []string{}

	if len(login.Email) == 0 {
		errs = append(errs, "Invalid Email")
	}

	if len(login.Name) == 0 {
		errs = append(errs, "Invalid Name")
	}

	if db.IsPasswordSecure(login.Password) == false {
		if len(login.Password) <= 7 {
			errs = append(errs, "Weak Password (shorter than 8)")
		}

		if login.Password == strings.ToLower(login.Password) {
			errs = append(errs, "Weak Password (lowercase)")
		}
	}

	if len(login.PhoneNumber) == 0 {
		errs = append(errs, "Invalid Phone Number")
	}

	address, err := mail.ParseAddress(strings.ToLower(login.Email))
	if err != nil {
		errs = append(errs, "Invalid Email Address")
	}

	if len(errs) > 0 {
		syrup.WriteJSON(w, http.StatusBadRequest, syrup.H{
			"errors": errs,
		})

		return
	}

	exists, err := db.Cols.Users.Find(db.M{
		"email": address.Address,
	}).Count()
	if err != nil {
		panic(err)
	}

	if exists > 0 {
		syrup.WriteJSON(w, http.StatusBadRequest, syrup.H{
			"errors": []string{"User Exists"},
		})

		return
	}

	userID := bson.NewObjectId()
	hash, salt := db.HashPassword(login.Password)
	if err := db.Cols.Users.Insert(db.M{
		"_id":                 userID,
		"password":            hash,
		"salt":                salt,
		"email":               address.Address,
		"name":                login.Name,
		"created":             time.Now(),
		"no_unspent_criminal": login.NoUnspentCriminal,
	}); err != nil {
		panic(err)
	}

	membership := db.UserMembership{
		UserID:      userID,
		PhoneNumber: login.PhoneNumber,
	}
	if err := db.Cols.Memberships.Insert(&membership); err != nil {
		panic(err)
	}

	encoded, err := config.Cookie.Encode("Maple Fleet", map[string]interface{}{
		"user": userID.Hex(),
	})
	if err != nil {
		panic(err)
	}

	if config.Config.Live {
		go func() {
			msg, _ := db.NewMail("info@maple.ai", "New Driver Signup", db.NewDriverAlert, map[string]interface{}{
				"Email": address.Address,
				"Name":  login.Name,
				"Time":  time.Now().Format("02-01-2006 15:04"),
			})
			if _, _, err := config.Mail.Send(msg); err != nil {
				panic(err)
			}
		}()
	}

	syrup.WriteJSON(w, http.StatusCreated, map[string]string{
		"token": encoded,
	})
}

func googleSignin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"code"`
	}
	if err := syrup.Bind(w, r, &body); err != nil {
		return
	}

	v := url.Values{}
	v.Add("code", body.Token)
	v.Add("client_id", config.Config.Google.ClientID)
	v.Add("client_secret", config.Config.Google.Secret)
	v.Add("redirect_uri", config.Config.Google.AuthRedirect)
	v.Add("grant_type", "authorization_code")

	resp, err := http.Post("https://www.googleapis.com/oauth2/v4/token", "application/x-www-form-urlencoded", bytes.NewReader([]byte(v.Encode())))
	defer resp.Body.Close()

	if err != nil {
		panic(err)
	}

	var response struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`

		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
		Token       string `json:"id_token"`
	}

	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&response); err != nil {
		panic(err)
	}

	if resp.StatusCode >= 400 {
		fmt.Println(response.Error, response.ErrorDescription)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp, err = http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + response.AccessToken)
	defer resp.Body.Close()

	if err != nil {
		panic(err)
	}

	var userProfile struct {
		ID            string `json:"id"`
		Email         string `json:"email"`
		VerifiedEmail bool   `json:"verified_email"`
		Name          string `json:"name"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
		Link          string `json:"link"`
		Picture       string `json:"picture"`
		Locale        string `json:"locale"`
		Hd            string `json:"hd"`
	}

	decoder = json.NewDecoder(resp.Body)
	if err := decoder.Decode(&userProfile); err != nil {
		panic(err)
	}

	if resp.StatusCode >= 400 || &userProfile == nil {
		fmt.Println(resp.StatusCode)
		return
	}

	var user db.User
	if err := db.Cols.Users.Find(db.M{
		"email": userProfile.Email,
	}).One(&user); err != nil && err != mgo.ErrNotFound {
		panic(err)
	}

	if !user.ID.Valid() {
		user.ID = bson.NewObjectId()

		if err := db.Cols.Users.Insert(db.M{
			"_id":     user.ID,
			"email":   userProfile.Email,
			"name":    userProfile.Name,
			"created": time.Now(),
		}); err != nil {
			panic(err)
		}
	}

	if err := db.Cols.Users.UpdateId(user.ID, db.M{"$set": db.M{"last_login": time.Now()}}); err != nil {
		panic(err)
	}

	encoded, err := config.Cookie.Encode("Maple Fleet", map[string]interface{}{
		"user": user.ID.Hex(),
	})
	if err != nil {
		panic(err)
	}

	syrup.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"token": encoded,
	})
}

func sendReset(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := syrup.Bind(w, r, &body); err != nil {
		return
	}

	var user db.User
	if err := db.Cols.Users.Find(db.M{"email": strings.ToLower(body.Email)}).One(&user); err != nil && err != mgo.ErrNotFound {
		panic(err)
	} else if err == mgo.ErrNotFound || user.Protected {
		syrup.WriteJSON(w, http.StatusBadRequest, map[string]string{
			"error": "User account does not exist",
		})
		return
	}

	b := make([]byte, 512)
	rand.Read(b)

	token := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b)
	if err := db.DB.C("password_resets").Insert(db.M{
		"user_id": user.ID,
		"token":   token,
		"expire":  time.Now().Add(15 * time.Minute),
	}); err != nil {
		panic(err)
	}

	msg, _ := db.NewMail(user.Email, db.PasswordResetSubject, db.PasswordReset, map[string]interface{}{
		"UserName": user.GetName(),
		"Google":   config.Config.Google,
		"Token":    token,
	})
	if _, _, err := config.Mail.Send(msg); err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusNoContent)
}

func doReset(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := syrup.Bind(w, r, &body); err != nil {
		panic(err)
	}

	if db.IsPasswordSecure(body.Password) == false {
		syrup.WriteJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Weak Password",
		})
		return
	}

	var reset db.M
	if err := db.DB.C("password_resets").Find(db.M{
		"token":  body.Token,
		"expire": db.M{"$gt": time.Now()},
	}).One(&reset); err != nil && err != mgo.ErrNotFound {
		panic(err)
	} else if err == mgo.ErrNotFound {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// reset password
	user, _ := db.FindUserByID(reset["user_id"].(bson.ObjectId))
	if err := user.UpdatePassword(body.Password); err != nil {
		panic(err)
	}

	db.DB.C("password_resets").RemoveAll(db.M{
		"user_id": user.ID,
	})

	w.WriteHeader(http.StatusNoContent)
}
