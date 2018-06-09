/*
 * (c) 2018, Caoimhe Chaos <caoimhechaos@protonmail.com>,
 *       Starship Factory. All rights reserved.
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
	textTemplate "text/template"

	"ancient-solutions.com/ancientauth"
	"github.com/starshipfactory/membersys"
)

// Handler object for displaying user takeout data.
type TakeoutOverviewHandler struct {
	auth                 *ancientauth.Authenticator
	database             *membersys.MembershipDB
	uniqueMemberTemplate *template.Template
}

// Serve the takeout console of the requestor.
func (m *TakeoutOverviewHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var agreement *membersys.MembershipAgreement
	var user string
	var err error

	if user = m.auth.GetAuthenticatedUser(req); user == "" {
		m.auth.RequestAuthorization(rw, req)
		return
	}

	agreement, err = m.database.GetMemberDetailByUsername(user)
	if err != nil {
		log.Print("Can't get membership agreement for ", user, ": ", err)
		rw.Header().Set("Content-type", "text/plain; charset=utf-8")
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Error retrieving membership agreement data"))
		return
	}

	err = m.uniqueMemberTemplate.ExecuteTemplate(rw, "memberdetail.html",
		agreement.GetMemberData())
	if err != nil {
		log.Print("Can't run membership detail template: ", err)
	}
}

// Handler object for downloading the membership agreement PDF.
type TakeoutPDFDownloadHandler struct {
	auth     *ancientauth.Authenticator
	database *membersys.MembershipDB
}

// Serve the membership agreement PDF of the requestor.
func (m *TakeoutPDFDownloadHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var agreement *membersys.MembershipAgreement
	var user string
	var err error

	if user = m.auth.GetAuthenticatedUser(req); user == "" {
		m.auth.RequestAuthorization(rw, req)
		return
	}

	agreement, err = m.database.GetMemberDetailByUsername(user)
	if err != nil {
		log.Print("Can't get membership agreement for ", user, ": ", err)
		rw.Header().Set("Content-type", "text/plain; charset=utf-8")
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Error retrieving membership agreement data"))
		return
	}

	if len(agreement.AgreementPdf) == 0 {
		rw.Header().Set("Content-type", "text/plain; charset=utf-8")
		rw.WriteHeader(http.StatusNotFound)
		rw.Write([]byte("Agreement PDF not found for " + user))
		return
	}

	rw.Header().Set("Content-type", "application/pdf")
	rw.WriteHeader(http.StatusOK)

	rw.Write(agreement.AgreementPdf)
}

// Handler object for downloading the user data as VCF.
type TakeoutVCFDownloadHandler struct {
	auth        *ancientauth.Authenticator
	database    *membersys.MembershipDB
	vcfTemplate *textTemplate.Template
}

// Serve the members own data in VCF standard.
func (m *TakeoutVCFDownloadHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var agreement *membersys.MembershipAgreement
	var user string
	var err error

	if user = m.auth.GetAuthenticatedUser(req); user == "" {
		m.auth.RequestAuthorization(rw, req)
		return
	}

	agreement, err = m.database.GetMemberDetailByUsername(user)
	if err != nil {
		log.Print("Can't get membership agreement for ", user, ": ", err)
		rw.Header().Set("Content-type", "text/plain; charset=utf-8")
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Error retrieving membership agreement data"))
		return
	}

	rw.Header().Set("Content-type", "text/vcard; charset=utf-8")
	rw.Header().Set("Content-disposition", "attachment; filename=\""+user+".vcf\"")
	rw.WriteHeader(http.StatusOK)

	err = m.vcfTemplate.Execute(rw, agreement.GetMemberData())
	if err != nil {
		log.Print("Error executing VCF template: ", err)
	}
}
