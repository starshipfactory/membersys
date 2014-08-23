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
	"time"

	"ancient-solutions.com/ancientauth"
)

type memberListType struct {
	Members   []*Member `json:"members"`
	CsrfToken string    `json:"csrf_token"`
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
	admingroup string
	auth       *ancientauth.Authenticator
	database   *MembershipDB
	pagesize   int32
	template   *template.Template
}

type TotalRecordList struct {
	Applicants []*MemberWithKey
	Members    []*Member
	Queue      []*MemberWithKey
	DeQueue    []*MemberWithKey
	Trash      []*MemberWithKey

	ApprovalCsrfToken  string
	RejectionCsrfToken string
	UploadCsrfToken    string
	CancelCsrfToken    string
	GoodbyeCsrfToken   string

	PageSize int32
}

// Serve the list of current membership applications to the requestor.
func (m *TotalListHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var all_records TotalRecordList
	var err error

	if m.auth.GetAuthenticatedUser(req) == "" {
		m.auth.RequestAuthorization(rw, req)
		return
	}

	if len(m.admingroup) > 0 && !m.auth.IsAuthenticatedScope(req, m.admingroup) {
		rw.Header().Set("Location", "/")
		rw.WriteHeader(http.StatusTemporaryRedirect)
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
	database   *MembershipDB
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
	database   *MembershipDB
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

	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte("{}"))
}
