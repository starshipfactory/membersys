/*
 * (c) 2014, Caoimhe Chaos <caoimhechaos@protonmail.com>,
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
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"ancient-solutions.com/ancientauth"
	"github.com/starshipfactory/membersys"
)

type memberListType struct {
	Members   []*membersys.Member `json:"members"`
	CsrfToken string              `json:"csrf_token"`
}

var memberGoodbyeURL *url.URL

func init() {
	var err error
	memberGoodbyeURL, err = url.Parse("/admin/api/goodbye-member")
	if err != nil {
		log.Fatal("Error parsing member goodbye URL: ", err)
	}
}

// Handler object for displaying the list of membership applications.
type TotalListHandler struct {
	admingroup           string
	auth                 *ancientauth.Authenticator
	database             *membersys.MembershipDB
	pagesize             int32
	template             *template.Template
	uniqueMemberTemplate *template.Template
}

type TotalRecordList struct {
	Applicants []*membersys.MemberWithKey
	Members    []*membersys.Member
	Queue      []*membersys.MemberWithKey
	DeQueue    []*membersys.MemberWithKey
	Trash      []*membersys.MemberWithKey

	ApprovalCsrfToken  string
	RejectionCsrfToken string
	UploadCsrfToken    string
	CancelCsrfToken    string
	GoodbyeCsrfToken   string

	PageSize int32
}

// Serve the list of current membership applications to the requestor.
func (m *TotalListHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var user string
	var all_records TotalRecordList
	var err error

	if user = m.auth.GetAuthenticatedUser(req); user == "" {
		m.auth.RequestAuthorization(rw, req)
		return
	}

	if len(m.admingroup) > 0 && !m.auth.IsAuthenticatedScope(req, m.admingroup) {
		var agreement *membersys.MembershipAgreement

		agreement, err = m.database.GetMemberDetailByUsername(user)
		if err != nil {
			log.Print("Can't get membership agreement for ", user, ": ", err)
			return
		}

		err = m.uniqueMemberTemplate.ExecuteTemplate(rw, "memberdetail.html",
			agreement.GetMemberData())
		if err != nil {
			log.Print("Can't run membership detail template: ", err)
		}

		return
	}

	all_records.Applicants, err = m.database.EnumerateMembershipRequests(
		req.FormValue("applicant_criterion"),
		req.FormValue("applicant_start"), m.pagesize)
	if err != nil {
		log.Print("Unable to list applicants from ",
			req.FormValue("applicant_start"), ": ", err)
	}

	all_records.Members, err = m.database.EnumerateMembers(
		req.FormValue("member_start"), m.pagesize)
	if err != nil {
		log.Print("Unable to list members from ",
			req.FormValue("member_start"), ": ", err)
	}

	all_records.Queue, err = m.database.EnumerateQueuedMembers(
		req.FormValue("queued_start"), m.pagesize)
	if err != nil {
		log.Print("Unable to list queued members from ",
			req.FormValue("queued_start"), ": ", err)
	}

	all_records.DeQueue, err = m.database.EnumerateDeQueuedMembers(
		req.FormValue("queued_start"), m.pagesize)
	if err != nil {
		log.Print("Unable to list dequeued members from ",
			req.FormValue("queued_start"), ": ", err)
	}

	all_records.Trash, err = m.database.EnumerateTrashedMembers(
		req.FormValue("trashed_start"), m.pagesize)
	if err != nil {
		log.Print("Unable to list trashed members from ",
			req.FormValue("trashed_start"), ": ", err)
	}

	all_records.ApprovalCsrfToken, err = m.auth.GenCSRFToken(
		req, applicantApprovalURL, 10*time.Minute)
	if err != nil {
		log.Print("Error generating approval CSRF token: ", err)
	}
	all_records.RejectionCsrfToken, err = m.auth.GenCSRFToken(
		req, applicantRejectionURL, 10*time.Minute)
	if err != nil {
		log.Print("Error generating rejection CSRF token: ", err)
	}
	all_records.UploadCsrfToken, err = m.auth.GenCSRFToken(
		req, applicantAgreementUploadURL, 10*time.Minute)
	if err != nil {
		log.Print("Error generating agreement upload CSRF token: ", err)
	}
	all_records.CancelCsrfToken, err = m.auth.GenCSRFToken(
		req, queueCancelURL, 10*time.Minute)
	if err != nil {
		log.Print("Error generating queue cancellation CSRF token: ", err)
	}
	all_records.GoodbyeCsrfToken, err = m.auth.GenCSRFToken(
		req, memberGoodbyeURL, 10*time.Minute)
	if err != nil {
		log.Print("Error generating member goodbye CSRF token: ", err)
	}

	all_records.PageSize = m.pagesize

	err = m.template.ExecuteTemplate(rw, "memberlist.html", all_records)
	if err != nil {
		log.Print("Error executing member list template: ", err)
	}
}

// Get a list of members.
type MemberListHandler struct {
	admingroup string
	auth       *ancientauth.Authenticator
	database   *membersys.MembershipDB
	pagesize   int32
}

func (m *MemberListHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var memlist memberListType
	var enc *json.Encoder
	var err error

	if !m.auth.IsAuthenticatedScope(req, m.admingroup) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	memlist.Members, err = m.database.EnumerateMembers(
		req.FormValue("start"), m.pagesize)
	if err != nil {
		log.Print("Error enumerating members: ", err)
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Error enumerating members: " + err.Error()))
		return
	}

	memlist.CsrfToken, err = m.auth.GenCSRFToken(req, memberGoodbyeURL,
		10*time.Minute)
	if err != nil {
		log.Print("Error generating CSRF token: ", err)
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Error generating CSRF token: " + err.Error()))
		return
	}

	rw.Header().Set("Content-Type", "application/json; encoding=utf8")
	enc = json.NewEncoder(rw)
	if err = enc.Encode(memlist); err != nil {
		log.Print("Error JSON encoding member list: ", err)
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Error encoding result: " + err.Error()))
		return
	}
}

// Object for removing members from the organization.
type MemberGoodbyeHandler struct {
	admingroup string
	auth       *ancientauth.Authenticator
	database   *membersys.MembershipDB
}

func (m *MemberGoodbyeHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var user string = m.auth.GetAuthenticatedUser(req)
	var reason string = req.PostFormValue("reason")
	var id string = req.PostFormValue("id")
	var ok bool
	var err error

	if user == "" {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	if len(m.admingroup) > 0 && !m.auth.IsAuthenticatedScope(req, m.admingroup) {
		rw.WriteHeader(http.StatusForbidden)
	}

	ok, err = m.auth.VerifyCSRFToken(req, req.PostFormValue("csrf_token"), false)
	if err != nil && err != ancientauth.CSRFToken_WeakProtectionError {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte(err.Error()))
		log.Print("Error verifying CSRF token: ", err)
		return
	}
	if !ok {
		rw.WriteHeader(http.StatusForbidden)
		rw.Write([]byte("CSRF token validation failed"))
		log.Print("Invalid CSRF token reveived")
		return
	}

	err = m.database.MoveMemberToTrash(id, user, reason)
	if err != nil {
		log.Print("Error moving member ", id, " to trash: ", err)
		rw.WriteHeader(http.StatusLengthRequired)
		rw.Write([]byte(err.Error()))
		return
	}

	rw.Header().Set("Content-Type", "application/json; encoding=utf8")
	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte("{}"))
}

// List details about a speific member.
type MemberDetailHandler struct {
	admingroup string
	auth       *ancientauth.Authenticator
	database   *membersys.MembershipDB
}

func (m *MemberDetailHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var user string = m.auth.GetAuthenticatedUser(req)
	var member *membersys.MembershipAgreement
	var memberid string = req.FormValue("email")
	var enc *json.Encoder
	var err error

	if user == "" {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	if len(memberid) == 0 {
		rw.WriteHeader(http.StatusLengthRequired)
		rw.Write([]byte("No email given"))
		return
	}

	member, err = m.database.GetMemberDetail(memberid)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Error fetching member details: " +
			err.Error()))
		return
	}

	if member.MemberData.GetUsername() != user && len(m.admingroup) > 0 &&
		!m.auth.IsAuthenticatedScope(req, m.admingroup) {
		rw.WriteHeader(http.StatusForbidden)
		rw.Write([]byte("Only admin users may look at other accounts"))
		return
	}

	// Trash the membership agreement, transmitting it over HTTP doesn't
	// make much sense.
	member.AgreementPdf = make([]byte, 0)

	// The password hash is off limits too.
	member.MemberData.Pwhash = nil

	rw.Header().Set("Content-Type", "application/json; encoding=utf8")
	enc = json.NewEncoder(rw)
	if err = enc.Encode(member); err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Error encoding JSON structure: " + err.Error()))
		return
	}
}

// Change one of a number of long fields.
type MemberLongFieldHandler struct {
	admingroup string
	auth       *ancientauth.Authenticator
	database   *membersys.MembershipDB
}

func (m *MemberLongFieldHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var memberid string = req.FormValue("email")
	var field string = req.FormValue("field")
	var value string = req.FormValue("value")
	var longValue uint64
	var err error

	if len(m.admingroup) > 0 && !m.auth.IsAuthenticatedScope(req, m.admingroup) {
		rw.WriteHeader(http.StatusForbidden)
		return
	}

	if len(memberid) == 0 || len(field) == 0 || len(value) == 0 {
		rw.WriteHeader(http.StatusLengthRequired)
		rw.Write([]byte("Required parameter missing"))
		return
	}

	longValue, err = strconv.ParseUint(value, 10, 64)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		rw.Write([]byte("Not a number: " + err.Error()))
		return
	}

	err = m.database.SetLongValue(memberid, field, longValue)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Error updating member details: " +
			err.Error()))
		return
	}

	rw.Header().Set("Content-Type", "application/json; encoding=utf8")
	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte("{}"))
}

// Change one of a number of boolean fields.
type MemberBoolFieldHandler struct {
	admingroup string
	auth       *ancientauth.Authenticator
	database   *membersys.MembershipDB
}

func (m *MemberBoolFieldHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var memberid string = req.FormValue("email")
	var field string = req.FormValue("field")
	var value string = req.FormValue("value")
	var boolValue bool
	var err error

	if len(m.admingroup) > 0 && !m.auth.IsAuthenticatedScope(req, m.admingroup) {
		rw.WriteHeader(http.StatusForbidden)
		return
	}

	if len(memberid) == 0 || len(field) == 0 || len(value) == 0 {
		rw.WriteHeader(http.StatusLengthRequired)
		rw.Write([]byte("Required parameter missing"))
		return
	}

	if value == "true" {
		boolValue = true
	} else if value == "false" {
		boolValue = false
	} else {
		rw.WriteHeader(http.StatusBadRequest)
		rw.Write([]byte("Value is not boolean: " + value))
		return
	}

	err = m.database.SetBoolValue(memberid, field, boolValue)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Error updating member details: " +
			err.Error()))
		return
	}

	rw.Header().Set("Content-Type", "application/json; encoding=utf8")
	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte("{}"))
}

// Change one of a number of text fields.
type MemberTextFieldHandler struct {
	admingroup string
	auth       *ancientauth.Authenticator
	database   *membersys.MembershipDB
}

func (m *MemberTextFieldHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var memberid string = req.FormValue("email")
	var field string = req.FormValue("field")
	var value string = req.FormValue("value")
	var err error

	if len(m.admingroup) > 0 && !m.auth.IsAuthenticatedScope(req, m.admingroup) {
		rw.WriteHeader(http.StatusForbidden)
		return
	}

	if len(memberid) == 0 || len(field) == 0 || len(value) == 0 {
		rw.WriteHeader(http.StatusLengthRequired)
		rw.Write([]byte("Required parameter missing"))
		return
	}

	err = m.database.SetTextValue(memberid, field, value)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Error updating member details: " +
			err.Error()))
		return
	}

	rw.Header().Set("Content-Type", "application/json; encoding=utf8")
	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte("{}"))
}

// Change the membership fee.
type MemberFeeHandler struct {
	admingroup string
	auth       *ancientauth.Authenticator
	database   *membersys.MembershipDB
}

func (m *MemberFeeHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var memberid string = req.FormValue("email")
	var fee_s string = req.FormValue("fee")
	var fee_yearly_s string = req.FormValue("fee_yearly")
	var fee uint64
	var fee_yearly bool
	var err error

	if len(m.admingroup) > 0 && !m.auth.IsAuthenticatedScope(req, m.admingroup) {
		rw.WriteHeader(http.StatusForbidden)
		return
	}

	if len(memberid) == 0 || len(fee_s) == 0 || len(fee_yearly_s) == 0 {
		rw.WriteHeader(http.StatusLengthRequired)
		rw.Write([]byte("Required parameter missing"))
		return
	}

	fee, err = strconv.ParseUint(fee_s, 10, 64)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		rw.Write([]byte("Not a number: " + err.Error()))
		return
	}

	if fee_yearly_s == "true" {
		fee_yearly = true
	} else if fee_yearly_s == "false" {
		fee_yearly = false
	} else {
		rw.WriteHeader(http.StatusBadRequest)
		rw.Write([]byte("Not a boolean"))
	}

	err = m.database.SetMemberFee(memberid, fee, fee_yearly)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Error updating membership fee: " +
			err.Error()))
		return
	}

	rw.Header().Set("Content-Type", "application/json; encoding=utf8")
	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte("{}"))
}
