package api

import (
	"net/http"
	"net/mail"
	"strings"

	"github.com/maple-ai/fleet-api/config"

	"github.com/maple-ai/syrup"
	"github.com/microcosm-cc/bluemonday"
)

func contactUs(w http.ResponseWriter, r *http.Request) {
	errs := []string{}
	var body struct {
		Name    string
		Email   string
		Message string
	}
	if err := syrup.Bind(w, r, &body); err != nil {
		return
	}

	policy := bluemonday.StrictPolicy()
	body.Name = policy.Sanitize(body.Name)
	body.Email = policy.Sanitize(body.Email)
	body.Message = policy.Sanitize(body.Message)

	if addr, err := mail.ParseAddress(strings.ToLower(body.Email)); err != nil {
		errs = append(errs, "Email address invalid")
	} else {
		body.Email = addr.Address
	}

	if len(body.Name) == 0 {
		errs = append(errs, "Name is required")
	}
	if len(body.Message) == 0 {
		errs = append(errs, "Message is required")
	}

	if len(errs) > 0 {
		syrup.WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
			"errors": errs,
		})
		return
	}

	msg := `
Name: ` + body.Name + `

Email: ` + body.Email + `

Message: ` + body.Message + `
	`

	message := config.Mail.NewMessage("Maple Fleet Mail <contact-us@maple.ai>", "New Message on Maple Fleet", msg, "info@maple.ai")
	if _, _, err := config.Mail.Send(message); err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusNoContent)
}
