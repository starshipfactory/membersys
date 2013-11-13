/*
 * (c) 2013, Tonnerre Lombard <tonnerre@ancient-solutions.com>,
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
	"net/http"
	"strings"
)

// Statistics
var num_requests *expvar.Int = expvar.NewInt("num-http-requests")
var num_submitted *expvar.Int = expvar.NewInt("num-successful-form-submissions")
var num_submit_errors *expvar.Map = expvar.NewMap("num-form-submission-errors")

type FormInputHandler struct {
	application_tmpl *template.Template
	print_tmpl       *template.Template
	passthrough      http.Handler
}

func (self *FormInputHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	num_requests.Add(1)

	// Pass JavaScript and CSS requests through to the passthrough handler.
	if strings.HasPrefix(req.URL.Path, "/css/") || strings.HasPrefix(req.URL.Path, "/js/") {
		self.passthrough.ServeHTTP(w, req)
	}

	var member *Member
	self.application_tmpl.Execute(w, member)
}
