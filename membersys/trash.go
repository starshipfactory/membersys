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

	"ancient-solutions.com/ancientauth"
	"github.com/starshipfactory/membersys"
)

// Object for displaying a list of deleted members.
type MemberTrashListHandler struct {
	admingroup string
	auth       *ancientauth.Authenticator
	database   membersys.MembershipDB
	pagesize   int32
}

func (m *MemberTrashListHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var memberlist []*membersys.MemberWithKey
	var enc *json.Encoder
	var err error

	if !m.auth.IsAuthenticatedScope(req, m.admingroup) {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	memberlist, err = m.database.EnumerateTrashedMembers(
		req.Context(), req.FormValue("start"), m.pagesize)
	if err != nil {
		log.Print("Error enumerating trashed members: ", err)
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Error enumerating trashed members: " + err.Error()))
		return
	}

	rw.Header().Set("Content-Type", "application/json; encoding=utf8")
	enc = json.NewEncoder(rw)
	if err = enc.Encode(memberlist); err != nil {
		log.Print("Error JSON encoding member list: ", err)
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Error encoding result: " + err.Error()))
		return
	}
}
