package main

import (
	"bytes"
	"net"
	"net/smtp"
	"text/template"
	"time"
)

type WelcomeMail struct {
	tmpl           *template.Template
	auth           smtp.Auth
	smtpserveraddr string
	from           string
	replyto        string
	subject        string
}

type welcomeTemplateData struct {
	Member  *Member
	From    string
	ReplyTo string
	Subject string
	Date    string
}

func NewWelcomeMail(config *WelcomeMailConfig) (*WelcomeMail, error) {
	var tmpl *template.Template
	var auth smtp.Auth
	var err error
	var host string

	host, _, err = net.SplitHostPort(config.GetSmtpServerAddress())
	if err != nil {
		return nil, err
	}

	if config.Username != nil && config.Password != nil {
		auth = smtp.PlainAuth(config.GetIdentity(), config.GetUsername(),
			config.GetPassword(), host)
	}
	tmpl, err = template.ParseFiles(config.GetMailTemplatePath())
	if err != nil {
		return nil, err
	}

	return &WelcomeMail{
		tmpl:           tmpl,
		auth:           auth,
		smtpserveraddr: config.GetSmtpServerAddress(),
		from:           config.GetFrom(),
		replyto:        config.GetReplyTo(),
		subject:        config.GetSubject(),
	}, nil
}

// Sends a welcome  e-mail to the new member.
func (w *WelcomeMail) SendMail(member *Member) error {
	var err error
	var recepients []string
	var messagebuffer = new(bytes.Buffer)

	// Save message in messagebuffer
	err = w.tmpl.Execute(messagebuffer, &welcomeTemplateData{
		Member:  member,
		From:    w.from,
		ReplyTo: w.replyto,
		Subject: w.subject,
		Date:    time.Now().Format(time.RFC1123Z), // "Mon, 02 Jan 2006 15:04:05 -0700" // RFC1123 with numeric zone
	})
	if err != nil {
		return err
	}

	recepients = []string{member.GetEmail()}

	err = smtp.SendMail(w.smtpserveraddr, w.auth, w.from, recepients, messagebuffer.Bytes())

	return err
}
