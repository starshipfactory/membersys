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
	"image/png"
	"log"
	"math/big"
	"net/http"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/gocql/gocql"
)

func MakeBarcode(rw http.ResponseWriter, req *http.Request) {
	var id = req.FormValue("id")
	var bigint *big.Int = big.NewInt(0)
	var code barcode.Barcode
	var uuid gocql.UUID
	var err error

	if id == "" {
		http.NotFound(rw, req)
		return
	}

	uuid, err = gocql.ParseUUID(id)
	if err != nil {
		log.Print("Error parsing UUID: ", err)
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Error parsing UUID: " + err.Error()))
		return
	}

	bigint.SetBytes(uuid.Bytes())
	id = bigint.String()

	code, err = code128.Encode(id)
	if err != nil {
		log.Print("Error generating barcode: ", err)
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Error generating barcode: " + err.Error()))
		return
	}

	code, err = barcode.Scale(code, code.Bounds().Max.X, 24*code.Bounds().Max.Y)
	if err != nil {
		log.Print("Error scaling barcode: ", err)
		rw.WriteHeader(http.StatusInternalServerError)
		rw.Write([]byte("Error scaling barcode: " + err.Error()))
		return
	}

	rw.Header().Set("Content-Type", "image/png")
	rw.Header().Set("Content-Disposition", "inline; filename="+uuid.String()+".png")
	err = png.Encode(rw, code)
	if err != nil {
		log.Print("Error writing out image: ", err)
		rw.Header().Set("Content-Type", "text/plain; charset=utf8")
		rw.Header().Set("Content-Disposition", "inline")
		rw.Write([]byte("Error writing out image: " + err.Error()))
	}
}
