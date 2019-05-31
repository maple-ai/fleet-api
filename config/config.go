/*
Package config provides CLI arguments and parses config file.

Asset stuff comes from go-bindata and should be ignored..
*/
package config

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"

	"github.com/gorilla/securecookie"
	// paypal "github.com/logpacker/PayPal-Go-SDK"
	mailgun "github.com/mailgun/mailgun-go"
	"github.com/stripe/stripe-go"
)

var (
	ScryptWorkFactor    = int(math.Pow(2, 15))
	ScryptBlockSize     = 8
	ScryptParallization = 1
)

var Config struct {
	MongoURL  string `json:"mongo_url"`
	MongoDB   string `json:"mongo_db"`
	SessionDb string `json:"session_db"`
	Live      bool   `json:"live"`

	CookieHash       string `json:"cookie_hash"`
	CookieEncryption string `json:"cookie_encryption"`

	Port       string `json:"port"`
	BugsnagKey string `json:"bugsnag_key"`
	Google     struct {
		ClientID     string `json:"client_id"`
		Secret       string `json:"secret"`
		AuthRedirect string `json:"auth_redirect"`
	} `json:"google"`

	Stripe struct {
		Secret string `json:"secret"`
		Pub    string `json:"pub"`
	} `json:"stripe"`

	GPS struct {
		Endpoint string `json:"endpoint"`
		Email    string `json:"email"`
		Password string `json:"password"`
	} `json:"gps"`

	Paypal struct {
		Account string `json:"account"`
		ID      string `json:"id"`
		Secret  string `json:"secret"`
	} `json:"paypal"`

	Mailgun struct {
		Domain string `json:"domain"`
		APIKey string `json:"api_key"`
	} `json:"mailgun"`
}
var Cookie *securecookie.SecureCookie

// var PaypalClient *paypal.Client
var Mail mailgun.Mailgun

func Parse() error {
	// stage := ""
	// if gin.Mode() == "release" {
	// 	stage = "production"
	// } else {
	// 	stage = "development"
	// }
	// bugsnag.Configure(bugsnag.Configuration{
	// 	APIKey:              Config.BugsnagKey,
	// 	ReleaseStage:        stage,
	// 	NotifyReleaseStages: []string{"production"},
	// 	// AppVersion:          Version,
	// 	// ProjectPackages:     []string{"github.com/maple-ai/rs/backend/**"},
	// })

	path := flag.String("config", "", "config path")
	flag.Parse()

	if path != nil && len(*path) > 0 {
		// load
		file, err := os.Open(*path)
		if err != nil {
			return err
		}

		if err := json.NewDecoder(file).Decode(&Config); err != nil {
			fmt.Println("Could not decode configuration!")
			return err
		}
	} else {
		// configFile, err := Asset("config.json")
		// if err != nil {
		// 	fmt.Println("Cannot Find configuration.")
		// 	return err
		// }

		// if err := json.Unmarshal(configFile, &Config); err != nil {
		// 	fmt.Println("Could not decode configuration!")
		// 	return err
		// }
	}

	if len(os.Getenv("PORT")) > 0 {
		Config.Port = ":" + os.Getenv("PORT")
	}

	if len(Config.Port) == 0 {
		Config.Port = ":3000"
	}

	var encryption []byte
	encryption = nil

	hash, _ := base64.StdEncoding.DecodeString(Config.CookieHash)
	if len(Config.CookieEncryption) > 0 {
		encryption, _ = base64.StdEncoding.DecodeString(Config.CookieEncryption)
	}

	Cookie = securecookie.New(hash, encryption)
	stripe.Key = Config.Stripe.Secret

	// Setup paypal
	// if c, err := paypal.NewClient(Config.Paypal.ID, Config.Paypal.Secret, paypal.APIBaseSandBox); err != nil {
	// 	panic(err)
	// } else {
	// 	c.SetLog(os.Stdout)
	// 	PaypalClient = c
	// }

	// Setup mailgun
	Mail = mailgun.NewMailgun(Config.Mailgun.Domain, Config.Mailgun.APIKey)

	return nil
}

func GetGPSAuthorization() string {
	str := Config.GPS.Email + ":" + Config.GPS.Password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(str))
}
