package db

import (
	"bytes"
	"text/template"

	"github.com/mailgun/mailgun-go"
	"github.com/maple-ai/fleet-api/config"
)

const MembershipSubmissionSubject = `Maple Fleet Driver Registration`
const MembershipSubmission = `
<style>* {font-size: 1rem;}</style>
Dear {{ .UserName }},

Thank you for registering your interest as a driver with Maple Fleet!

We have received your details and someone from our on-boarding team will contact you in the next few days for a phone interview.

We look forward to welcoming you as a Maple Fleet driver!

Kind regards,<br/>
Maple Fleet Team

[maple.ai](https://maple.ai)

![maple-fleet](https://maple.ai/front-page/sf-logo.png)
`

const MembershipInterviewSubject = `Maple Fleet On Boarding Session`
const MembershipInterviewRescheduledSubject = `Maple Fleet On Boarding Session rescheduled`
const MembershipInterview = `
<style>* {font-size: 1rem;}</style>
Dear {{ .UserName }},

It was great to speak to you just now. We are looking forward to meeting you on {{ .Date }} at Maple Fleet's hub:

Singapore
London

Please make sure you bring your:

- driver's licence;
- CBT Certificate;
- a copy of your delivery insurance policy (if you have one);
- evidence of your right to work in the UK; and
- Proof of address (bank statement, phone bill, council bill etc).

If you need to reschedule your on boarding session, please contact us at info@maple.ai.

Otherwise, we'll see you soon!

Kind regards,<br/>
Maple Fleet Team

[maple.ai](https://www.maple.ai)

![maple-fleet](https://fleet.maple.ai)
`

const MembershipAcceptedSubject = `Maple Fleet Membership Accepted!`
const MembershipAccepted = `
<style>* {font-size: 1rem;}</style>
Dear {{ .UserName }},

Welcome to Maple Fleet! We are thrilled to have you on our platform and look forward to working with you soon.

Your Maple Fleet application is now complete and your membership number is: {{ .MembershipID }}.

We will be in touch shortly to link you up to our scheduling and dispatch systems.

Congratulations!

Kind regards,<br/>
Maple Fleet Team

[maple.ai](https://www.maple.ai)

![maple-fleet](https://www.maple.ai/)
`

const MembershipRejectedSubject = `Maple Fleet Membership Unsuccessful`
const MembershipRejected = `
<style>* {font-size: 1rem;}</style>
Dear {{ .UserName }},

We are sorry to inform you that your application to become a member for Maple Fleet has been unsuccessful.

Kind regards,<br/>
Maple Fleet Team

[maple.ai](https://maple.ai)

![maple-fleet](https://maple.ai/front-page/sf-logo.png)
`

const UserPayoutSubject = `Maple Fleet Payment Confirmation`
const UserPayout = `
<style>* {font-size: 1rem;}</style>
Dear {{ .UserName }},

Thank you for driving for us this week!

We have now remitted your weekly wages to you nominated bank account by PayPal.

If you wish to verify the amount you have received, please log into your Maple Fleet member profile and go to the Transactions tab.

If you have any queries, please contact us at info@maple.ai

We look forward to seeing you next week.

Kind regards,<br/>
Maple Fleet Team

[maple.ai](https://maple.ai)

![maple-fleet](https://maple.ai/front-page/sf-logo.png)
`

const PasswordResetSubject = `Maple Fleet Password Reset`
const PasswordReset = `
<style>* {font-size: 1rem;}</style>
Dear {{ .UserName }},

To reset your password, please click the link below. The link is valid for 15 minutes since reset instruction was sent.

{{ .Google.AuthRedirect }}/password-reset/{{ .Token }}

If this wasn't you, please delete this email.

Kind regards,<br/>
Maple Fleet Team

[maple.ai](https://maple.ai)

![maple-fleet](https://maple.ai/front-page/sf-logo.png)
`

const UserBanSubject = `Maple Fleet Account Suspended`
const UserBan = `
Hello,

Unfortunately we have had to temporarily ban your membership to Maple Fleet.

This may be for a number of reasons so please contact us on the details below with any questions:

07802 884844<br/>
info@maple.ai

Once this is sorted out, we’ll have you back on the road in no time.

Maple Fleet

[maple.ai](https://maple.ai)

![maple-fleet](https://maple.ai/front-page/sf-logo.png)
`

const UserUnbanSubject = `Maple Fleet Account Re-instated`
const UserUnban = `
Hello,

Everything looks good to go now; so your membership to Maple Fleet has now been re-instated.

We look forward to seeing you on the road again soon.

Maple Fleet

[maple.ai](https://maple.ai)

![maple-fleet](https://maple.ai/front-page/sf-logo.png)
`

const NewDriverAlert = `

New user sign up alert
----------------------

    Name: {{ .Name }}
    Email: {{ .Email }}
    Time: {{ .Time }}

`

func NewMail(to string, subject string, content string, data interface{}) (*mailgun.Message, error) {
	tpl := template.Must(template.New("email").Parse(content))
	buf := new(bytes.Buffer)
	if err := tpl.Execute(buf, data); err != nil {
		return nil, err
	}

	message := config.Mail.NewMessage("Maple Fleet <info@maple.ai>", subject, "", to)
	// message.SetHtml(string(blackfriday.MarkdownCommon([]byte(buf.String()))))

	return message, nil
}
