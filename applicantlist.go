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
	"database/cassandra"
	"encoding/json"
	"io/ioutil"
	"log"
	"math/big"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"

	"ancient-solutions.com/ancientauth"
	"code.google.com/p/goprotobuf/proto"
)

type applicantListType struct {
	Applicants               []*MemberWithKey `json:"applicants"`
	ApprovalCsrfToken        string           `json:"approval_csrf_token"`
	RejectionCsrfToken       string           `json:"rejection_csrf_token"`
	AgreementUploadCsrfToken string           `json:"agreement_upload_csrf_token"`
}

type ApplicantListHandler struct {
	admingroup string
	auth       *ancientauth.Authenticator
	database   *MembershipDB
	pagesize   int32
}

var applicantApprovalURL *url.URL
var applicantRejectionURL *url.URL
var applicantAgreementUploadURL *url.URL

func init() {
	var err error
	applicantApprovalURL, err = url.Parse("/admin/api/accept")
	if err != nil {
		log.Fatal("Error parsing static approval URL: ", err)
	}
	applicantRejectionURL, err = url.Parse("/admin/api/reject")
	if err != nil {
		log.Fatal("Error parsing static rejection URL: ", err)
	}
	applicantAgreementUploadURL, err = url.Parse("/admin/api/agreement-upload")
	if err != nil {
		log.Fatal("Error parsing static agreement upload URL: ", err)
	}
}

// Output a JSON list of all applicants currently waiting to become members.
func (a *ApplicantListHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var applist applicantListType
	var enc *json.Encoder
	var err error

	if !a.auth.IsAuthenticatedScope(req, a.admingroup) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	if req.FormValue("single") == "true" && len(req.FormValue("start")) > 0 {
		var memberreq *MembershipAgreement
		var mwk *MemberWithKey
		var bigint *big.Int = big.NewInt(0)
		var uuid cassandra.UUID
		var ok bool
		bigint, ok = bigint.SetString(req.FormValue("start"), 10)
		if !ok {
			rw.WriteHeader(http.StatusInternalServerError)
			rw.Write([]byte("Unable to parse " + req.FormValue("start") +
				" as a number"))
			return
		}
		if bigint.BitLen() == 128 {
			uuid = cassandra.UUIDFromBytes(bigint.Bytes())
			memberreq, _, err = a.database.GetMembershipRequest(
				uuid.String(), "application", "applicant:")
			if err != nil {
				rw.WriteHeader(http.StatusInternalServerError)
				rw.Write([]byte("Unable to retrieve the membership request " +
					uuid.String() + ": " + err.Error()))
				return
			}
			mwk = new(MemberWithKey)
			mwk.Key = uuid.String()
			proto.Merge(&mwk.Member, memberreq.GetMemberData())
			applist.Applicants = []*MemberWithKey{mwk}
		}
	} else {
		applist.Applicants, err = a.database.EnumerateMembershipRequests(
			req.FormValue("criterion"), req.FormValue("start"), a.pagesize)
		if err != nil {
			log.Print("Error enumerating applications: ", err)
			rw.WriteHeader(http.StatusInternalServerError)
			rw.Write([]byte("Error enumerating applications: " + err.Error()))
			return
		}
	}

	applist.AgreementUploadCsrfToken, err = a.auth.GenCSRFToken(
		req, applicantAgreementUploadURL, 10*time.Minute)
	if err != nil {
		log.Print("Error generating CSRF token: ", err)
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Error generating CSRF token: " + err.Error()))
		return
	}

	applist.ApprovalCsrfToken, err = a.auth.GenCSRFToken(
		req, applicantApprovalURL, 10*time.Minute)
	if err != nil {
		log.Print("Error generating CSRF token: ", err)
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Error generating CSRF token: " + err.Error()))
		return
	}

	applist.RejectionCsrfToken, err = a.auth.GenCSRFToken(
		req, applicantRejectionURL, 10*time.Minute)
	if err != nil {
		log.Print("Error generating CSRF token: ", err)
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Error generating CSRF token: " + err.Error()))
		return
	}

	rw.Header().Set("Content-Type", "application/json; encoding=utf8")
	enc = json.NewEncoder(rw)
	if err = enc.Encode(applist); err != nil {
		log.Print("Error JSON encoding member list: ", err)
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Error encoding result: " + err.Error()))
		return
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
	var ok bool
	var err error

	if user == "" {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	if len(m.admingroup) > 0 && !m.auth.IsAuthenticatedScope(req, m.admingroup) {
		rw.WriteHeader(http.StatusForbidden)
		rw.Write([]byte("User not authorized for this service"))
		return
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

// Object for uploading membership agreements.
type MemberAgreementUploadHandler struct {
	admingroup string
	auth       *ancientauth.Authenticator
	database   *MembershipDB
}

func (m *MemberAgreementUploadHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var user string = m.auth.GetAuthenticatedUser(req)
	var id string = req.FormValue("uuid")
	var mf multipart.File
	var agreement_data []byte
	var ok bool
	var err error

	if user == "" {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	if len(m.admingroup) > 0 && !m.auth.IsAuthenticatedScope(req, m.admingroup) {
		rw.WriteHeader(http.StatusForbidden)
	}

	req.URL.RawQuery = ""
	req.ParseMultipartForm(5 * 1048576)

	ok, err = m.auth.VerifyCSRFToken(req, req.FormValue("csrf_token"), false)
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

	mf, _, err = req.FormFile("0")
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		rw.Write([]byte("Unable to retrieve uploaded file: " + err.Error()))
		log.Print("Unable to retrieve uploaded file: ", err)
		return
	}

	agreement_data, err = ioutil.ReadAll(mf)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Error reading in agreement data: " + err.Error()))
		log.Print("Error reading in agreement data: ", err)
		return
	}

	mf.Close()

	err = m.database.StoreMembershipAgreement(id, agreement_data)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Error storing membership agreement: " + err.Error()))
		log.Print("Error storing membership agreement: ", err)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte("{}"))
}
