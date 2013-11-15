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
	"expvar"
	"html/template"
	"log"
	"net/http"
	"regexp"
	"strings"
)

var requiredFields []string = []string{
	"name",
	"address",
	"city",
	"zip",
	"country",
}

// Statistics
var numRequests *expvar.Int = expvar.NewInt("num-http-requests")
var numSubmitted *expvar.Int = expvar.NewInt("num-successful-form-submissions")
var numSubmitErrors *expvar.Map = expvar.NewMap("num-form-submission-errors")

var emailRe *regexp.Regexp
var phoneRe *regexp.Regexp

type FormInputHandler struct {
	applicationTmpl *template.Template
	printTmpl       *template.Template
	passthrough     http.Handler
}

type FormInputData struct {
	MemberData *Member
	CommonErr  string
	FieldErr   map[string]string
}

func (self *FormInputHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var err error
	var data FormInputData
	var field string
	var ok bool
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

	for _, field = range requiredFields {
		if len(req.PostFormValue("mr["+field+"]")) <= 0 {
			numSubmitErrors.Add("no-"+field, 1)
			ok = false
		}
	}

	if !emailRe.MatchString(req.PostFormValue("mr[email]")) {
		if len(req.PostFormValue("mr[email]")) > 0 {
			data.FieldErr["email"] = "Mailadresse sollte im Format a@b.ch sein"
			numSubmitErrors.Add("bad-email-format", 1)
		} else {
			numSubmitErrors.Add("no-email", 1)
		}
		ok = false
	}

	if len(req.PostFormValue("mr[telephone]")) > 0 &&
		!phoneRe.MatchString("mr[telephone]") {
		data.FieldErr["telephone"] = "Telephonnummer sollte im Format +41 79 123 45 67 sein"
		numSubmitErrors.Add("bad-phone-format", 1)
		ok = false
	}

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
