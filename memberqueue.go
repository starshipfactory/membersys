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
	"log"
	"net/http"
	"net/url"
	"time"

	"ancient-solutions.com/ancientauth"
)

type queueListType struct {
	Queued    []*MemberWithKey `json:"queued"`
	CsrfToken string           `json:"csrf_token"`
}

// Object for getting a list of currently queued members.
type MemberQueueListHandler struct {
	admingroup string
	auth       *ancientauth.Authenticator
	database   *MembershipDB
	pagesize   int32
}

var queueCancelURL *url.URL

func init() {
	var err error
	queueCancelURL, err = url.Parse("/admin/api/cancel-queued")
	if err != nil {
		log.Fatal("Error parsing queue cancellation URL: ", err)
	}
}

func (m *MemberQueueListHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var qlist queueListType
	var enc *json.Encoder
	var err error

	if !m.auth.IsAuthenticatedScope(req, m.admingroup) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	qlist.Queued, err = m.database.EnumerateQueuedMembers(
		req.FormValue("start"), m.pagesize)
	if err != nil {
		log.Print("Error enumerating membership queue: ", err)
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Error enumerating membership queue: " + err.Error()))
		return
	}

	qlist.CsrfToken, err = m.auth.GenCSRFToken(req, queueCancelURL,
		10*time.Minute)
	if err != nil {
		log.Print("Error generating CSRF token: ", err)
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Error generating CSRF token: " + err.Error()))
		return
	}

	rw.Header().Set("Content-Type", "application/json; encoding=utf8")
	enc = json.NewEncoder(rw)
	if err = enc.Encode(qlist); err != nil {
		log.Print("Error JSON encoding member list: ", err)
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Error encoding result: " + err.Error()))
		return
	}
}

// Object for cancelling a queued future member.
type MemberQueueCancelHandler struct {
	admingroup string
	auth       *ancientauth.Authenticator
	database   *MembershipDB
}

func (m *MemberQueueCancelHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var user string = m.auth.GetAuthenticatedUser(req)
	var id string = req.PostFormValue("uuid")
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

	err = m.database.MoveQueuedRecordToTrash(id, user)
	if err != nil {
		log.Print("Error moving queued record ", id, " to trash: ", err)
		rw.WriteHeader(http.StatusLengthRequired)
		rw.Write([]byte(err.Error()))
		return
	}

	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte("{}"))
}
