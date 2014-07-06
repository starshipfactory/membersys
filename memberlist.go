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
	"html/template"
	"log"
	"net/http"

	"ancient-solutions.com/ancientauth"
)

// Handler object for displaying the list of membership applications.
type ApplicantListHandler struct {
	admingroup string
	auth       *ancientauth.Authenticator
	database   *MembershipDB
	pagesize   int32
	template   *template.Template
}

// Serve the list of current membership applications to the requestor.
func (m *ApplicantListHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var applications []*MemberWithKey
	var err error

	if m.auth.GetAuthenticatedUser(req) == "" {
		m.auth.RequestAuthorization(rw, req)
		return
	}

	if len(m.admingroup) > 0 && !m.auth.IsAuthenticatedScope(req, m.admingroup) {
		rw.Header().Set("Location", "/")
		rw.WriteHeader(http.StatusTemporaryRedirect)
	}

	applications, err = m.database.EnumerateMembershipRequests(
		req.FormValue("criterion"), req.FormValue("start"), m.pagesize)
	if err != nil {
		log.Print("Unable to list members from ", req.FormValue("start"), ": ", err)
	}

	err = m.template.ExecuteTemplate(rw, "memberlist.html", applications)
	if err != nil {
		log.Print("Error executing member list template: ", err)
	}
}

// Object for approving membership applications.
type MemberAcceptHandler struct {
	admingroup string
	auth       *ancientauth.Authenticator
	database   *MembershipDB
}

func (m *MemberAcceptHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var user string = m.auth.GetAuthenticatedUser(req)
	var id string = req.PostFormValue("uuid")
	var err error

	if user == "" {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	if len(m.admingroup) > 0 && !m.auth.IsAuthenticatedScope(req, m.admingroup) {
		rw.WriteHeader(http.StatusForbidden)
	}

	err = m.database.MoveApplicantToNewMember(id, user)
	if err != nil {
		log.Print("Error moving applicant ", id, " to new user: ", err)
		rw.WriteHeader(http.StatusLengthRequired)
		rw.Write([]byte(err.Error()))
		return
	}

	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte("{}"))
}

// Object for rejecting membership applications.
type MemberRejectHandler struct {
	admingroup string
	auth       *ancientauth.Authenticator
	database   *MembershipDB
}

func (m *MemberRejectHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var user string = m.auth.GetAuthenticatedUser(req)
	var id string = req.PostFormValue("uuid")
	var err error

	if m.auth.GetAuthenticatedUser(req) == "" {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	if len(m.admingroup) > 0 && !m.auth.IsAuthenticatedScope(req, m.admingroup) {
		rw.WriteHeader(http.StatusForbidden)
	}

	err = m.database.MoveApplicantToTrash(id, user)
	if err != nil {
		log.Print("Error moving applicant ", id, " to trash: ", err)
		rw.WriteHeader(http.StatusLengthRequired)
		rw.Write([]byte(err.Error()))
		return
	}

	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte("{}"))
}
