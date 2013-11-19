/*
 * (c) 2013, Caoimhe Chaos <caoimhechaos@protonmail.com>,
 *	     Starship Factory. All rights reserved.
 *
 * Redistribution and use in source  and binary forms, with or without
 * modification, are permitted  provided that the following conditions
 * are met:
 *
 * * Redistributions of  source code  must retain the  above copyright
 *   notice, this list of conditions and the following disclaimer.
 * * Redistributions in binary form must reproduce the above copyright
 *   notice, this  list of conditions and the  following disclaimer in
 *   the  documentation  and/or  other  materials  provided  with  the
 *   distribution.
 * * Neither  the name  of the Starship Factory  nor the  name  of its
 *   contributors may  be used to endorse or  promote products derived
 *   from this software without specific prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
 * "AS IS"  AND ANY EXPRESS  OR IMPLIED WARRANTIES  OF MERCHANTABILITY
 * AND FITNESS  FOR A PARTICULAR  PURPOSE ARE DISCLAIMED. IN  NO EVENT
 * SHALL THE COPYRIGHT OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT,
 * INDIRECT, INCIDENTAL, SPECIAL,  EXEMPLARY, OR CONSEQUENTIAL DAMAGES
 * (INCLUDING, BUT NOT LIMITED  TO, PROCUREMENT OF SUBSTITUTE GOODS OR
 * SERVICES; LOSS OF USE,  DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
 * HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT,
 * STRICT  LIABILITY,  OR  TORT  (INCLUDING NEGLIGENCE  OR  OTHERWISE)
 * ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED
 * OF THE POSSIBILITY OF SUCH DAMAGE.
 */

package main

import (
	"crypto/sha1"
	"encoding/base64"
	"expvar"
	"hash"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var fmap = template.FuncMap{
	"html": template.HTMLEscaper,
	"url":  UserInputFormatter,
}

func UserInputFormatter(v ...interface{}) string {
	return template.HTMLEscapeString(url.QueryEscape(v[0].(string)))
}

// Statistics.
var numRequests *expvar.Int = expvar.NewInt("num-http-requests")
var numSubmitted *expvar.Int = expvar.NewInt("num-successful-form-submissions")
var numSubmitErrors *expvar.Map = expvar.NewMap("num-form-submission-errors")

// Regular expressions for verification of the email and phone number fields.
var emailRe *regexp.Regexp
var phoneRe *regexp.Regexp

// Data type for the HTTP handler which takes the requests. We require the
// templates and a passthrough object for static content requests, so we
// need to hold some state.
type FormInputHandler struct {
	applicationTmpl *template.Template
	printTmpl       *template.Template
	passthrough     http.Handler
}

// Data used by the HTML template. Contains not just data entered so far,
// but also some error texts in case there was a problem submitting data.
type FormInputData struct {
	MemberData *Member
	Comment    string
	CommonErr  string
	FieldErr   map[string]string
}

// Parse the form data from the membership signup form and verify that it
// can be considered acceptable. If the data looks correct, return the
// print template for the user to sign and send in.
func (self *FormInputHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var err error
	var data FormInputData
	var fee float64
	var ok bool = true
	numRequests.Add(1)

	// Pass JavaScript and CSS requests through to the passthrough handler.
	if strings.HasPrefix(req.URL.Path, "/css/") ||
		strings.HasPrefix(req.URL.Path, "/js/") ||
		req.URL.Path == "/favicon.ico" {
		self.passthrough.ServeHTTP(w, req)
		return
	}

	data.FieldErr = make(map[string]string)

	if err = req.ParseForm(); err != nil {
		data.CommonErr = err.Error()
		numSubmitErrors.Add(err.Error(), 1)
		self.applicationTmpl.Execute(w, data)
		return
	}

	// No data entered: the user is probably just going to the web site
	// for the first time, so data validation is useless.
	if len(req.PostForm) == 0 {
		self.applicationTmpl.Execute(w, data)
		return
	}

	// You might think that it would be a good idea to split the name
	// field into first and last name, which might even work for this
	// specific, localized use case, but it is bad practice, because
	// some countries don't have the concept of last names, and would
	// set a bad precedent for people reading and using this code.
	var name string = req.PostFormValue("mr[name]")
	if len(name) <= 0 {
		numSubmitErrors.Add("no-name", 1)
		data.FieldErr["name"] = "Ein Name ist erforderlich"
	} else {
		data.MemberData.Name = &name
	}

	// The same applies to the address. There just is no globally common
	// format for home addresses, not everything has a house number, and
	// we don't want to encourage people to think so.
	var address string = req.PostFormValue("mr[address]")
	if len(address) <= 0 {
		numSubmitErrors.Add("no-street", 1)
		data.FieldErr["address"] = "Eine Adresse ist erforderlich"
	} else {
		data.MemberData.Street = &address
	}

	var city string = req.PostFormValue("mr[city]")
	if len(city) <= 0 {
		numSubmitErrors.Add("no-city", 1)
		data.FieldErr["city"] = "Ein Wohnort ist erforderlich"
	} else {
		data.MemberData.Street = &city
	}

	// As for zip codes, you don't even have to go to India to find out
	// why enforcing a format there doesn't work. Many people around
	// here believe that a 4-5 digit number is enough to represent a zip
	// code. But then the Netherlands have zip codes like «4201 EB»
	// (where EB is part of the zip code). If you consider adding an
	// exception for this, please note that British zip codes look like
	// «G1 1PP». The only realistic way to deal with these is to allow
	// free text for zip codes.
	var zip string = req.PostFormValue("mr[zip]")
	if len(zip) <= 0 {
		numSubmitErrors.Add("no-zip", 1)
		data.FieldErr["zip"] = "Eine Postleitzahl ist erforderlich"
	} else {
		data.MemberData.Zipcode = &zip
	}

	// The country could arguably be a list.
	var country string = req.PostFormValue("mr[country]")
	if len(country) <= 0 {
		numSubmitErrors.Add("no-country", 1)
		data.FieldErr["country"] = "Ein Wohnland ist erforderlich"
	} else {
		data.MemberData.Country = &country
	}

	var email string = req.PostFormValue("mr[email]")
	if !emailRe.MatchString(email) {
		if len(email) > 0 {
			data.FieldErr["email"] = "Mailadresse sollte im Format a@b.ch sein"
			numSubmitErrors.Add("bad-email-format", 1)
		} else {
			data.FieldErr["email"] = "Muss angegeben werden"
			numSubmitErrors.Add("no-email", 1)
		}
		ok = false
	} else {
		data.MemberData.Email = &email
	}

	var phone string = req.PostFormValue("mr[telephone]")
	if len(phone) > 0 && !phoneRe.MatchString(phone) {
		data.FieldErr["telephone"] = "Telephonnummer sollte im Format +41 79 123 45 67 sein"
		numSubmitErrors.Add("bad-phone-format", 1)
		ok = false
	} else {
		data.MemberData.Phone = &phone
	}

	// TODO(caoimhe): Verify the user name field.
	var username string = req.PostFormValue("mr[username]")
	if len(username) > 0 {
		data.MemberData.Username = &username
	}

	var pw string = req.PostFormValue("mr[password]")
	if pw != req.PostFormValue("mr[passwordConfirm]") {
		data.FieldErr["password"] = "Passworte stimmen nicht überein"
		numSubmitErrors.Add("password-mismatch", 1)
		ok = false
	} else {
		var h hash.Hash = sha1.New()
		h.Write([]byte(pw))
		pw = "{SHA}" + base64.StdEncoding.EncodeToString(h.Sum([]byte{}))
		data.MemberData.Pwhash = &pw
	}

	if req.PostFormValue("mr[statutes]") != "accepted" {
		data.FieldErr["statutes"] = "Statuten müssen akzeptiert werden"
		numSubmitErrors.Add("statutes-not-accepted", 1)
		ok = false
	}

	if req.PostFormValue("mr[ipay]") != "accepted" {
		data.FieldErr["ipay"] = "Zahlungsbereitschaft ist notwendig"
		numSubmitErrors.Add("payment-not-accepted", 1)
		ok = false
	}

	if req.PostFormValue("mr[rules]") != "accepted" {
		data.FieldErr["rules"] = "Reglement muss akzeptiert werden"
		numSubmitErrors.Add("rules-not-accepted", 1)
		ok = false
	}

	if req.PostFormValue("mr[gt18]") != "yes" {
		data.FieldErr["gt18"] = "Man muss mindestens 18 Jahre sein, um uns beizutreten"
		numSubmitErrors.Add("not-gt18", 1)
		ok = false
	}

	if len(req.PostFormValue("mr[customFee]")) > 0 {
		fee, err = strconv.ParseFloat(req.PostFormValue("mr[customFee]"), 64)
		if err == strconv.ErrRange {
			data.FieldErr["customFee"] = "Der Betrag ist irgendwie etwas gross/klein, oder?"
			numSubmitErrors.Add("fee-out-of-range", 1)
			ok = false
		} else if err == strconv.ErrSyntax {
			data.FieldErr["customFee"] = "Der Mitgliedsbeitrag kann nicht als Zahl identifiziert werden"
			numSubmitErrors.Add("fee-not-a-number", 1)
			log.Print("Unable to parse ", req.PostFormValue("mr[customFee]"),
				" as a valid fee")
			ok = false
		} else if err != nil {
			// No idea? This shouldn't really happen.
			data.FieldErr["customFee"] = err.Error()
			log.Print("Error converting ", req.PostFormValue("mr[customFee]"),
				" to a number: ", err)
			ok = false
		}
	}
	if req.PostFormValue("mr[fee]") == "custom" {
		if fee < 50 && req.PostFormValue("mr[reduction]") != "requested" {
			data.FieldErr["customFee"] = "Für einen Betrag unter 50 CHF muss eine Ermässigung beantragt werden"
			numSubmitErrors.Add("low-fee-without-reduction", 1)
			ok = false
		} else if len(req.PostFormValue("mr[customFee]")) <= 0 {
			data.FieldErr["customFee"] = "Die Angabe eines Mitgliedsbeitrages ist notwendig"
			ok = false
		} else {
			var intfee uint64 = uint64(fee)
			data.MemberData.Fee = &intfee
		}
	} else if req.PostFormValue("mr[fee]") != "SFr. 50.--" {
		data.FieldErr["fee"] = "Unbekannter Wert für den Mitgliedsbeitrag"
		numSubmitErrors.Add("unknown-fee-value", 1)
		ok = false
	} else {
		var intfee uint64 = 50
		data.MemberData.Fee = &intfee
	}

	data.Comment = req.PostFormValue("mr[comments]")

	if ok {
		numSubmitted.Add(1)
		err = self.printTmpl.Execute(w, data)
		if err != nil {
			log.Print("Error executing print template: ", err)
		}
	} else {
		err = self.applicationTmpl.Execute(w, data)
		if err != nil {
			log.Print("Error executing request form template: ", err)
		}
	}
}

func init() {
	emailRe = regexp.MustCompile(`^[A-Za-z0-9-_\.]+@[A-Za-z0-9-_\.]+$`)
	phoneRe = regexp.MustCompile(`^\+?[0-9 -\.]+$`)
}
