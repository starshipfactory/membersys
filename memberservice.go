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
	"flag"
	"html/template"
	"log"
	"net/http"
	"os"

	"ancient-solutions.com/doozer/exportedservice"
)

func main() {
	var help bool
	var bindto, template_dir string
	var lockserv, lockboot, servicename string
	var application_tmpl, print_tmpl *template.Template
	var exporter *exportedservice.ServiceExporter
	var err error

	flag.BoolVar(&help, "help", false, "Display help")
	flag.StringVar(&bindto, "bind", "127.0.0.1:8080",
		"The address to bind the web server to")
	flag.StringVar(&lockserv, "lockserver-uri",
		os.Getenv("DOOZER_URI"),
		"URI of a Doozer cluster to connect to")
	flag.StringVar(&template_dir, "template-dir", "",
		"Path to the directory with the HTML templates")
	flag.StringVar(&lockboot, "lockserver-boot-uri",
		os.Getenv("DOOZER_BOOT_URI"),
		"Boot URI to resolve the Doozer cluster name (if required)")
	flag.StringVar(&servicename, "service-name",
		"", "Service name to publish as to the lock server")
	flag.Parse()

	if help {
		flag.Usage()
		os.Exit(1)
	}

	if len(template_dir) <= 0 {
		log.Fatal("The --template-dir flag must not be empty")
	}

	// Load and parse the HTML templates to be displayed.
	application_tmpl, err = template.ParseFiles(template_dir + "/form.html")
	if err != nil {
		log.Fatal("Unable to parse form template: ", err)
	}
	application_tmpl.Funcs(fmap)

	print_tmpl, err = template.ParseFiles(template_dir + "/printlayout.html")
	if err != nil {
		log.Fatal("Unable to parse print layout template: ", err)
	}
	print_tmpl.Funcs(fmap)

	// Register the URL handler to be invoked.
	http.Handle("/", &FormInputHandler{
		applicationTmpl: application_tmpl,
		printTmpl:       print_tmpl,
		passthrough:     http.FileServer(http.Dir(template_dir)),
	})

	// If a lock server was specified, attempt to use an anonymous port as
	// a Doozer exported HTTP service. Otherwise, just bind to the address
	// given in bindto, for debugging etc.
	if len(lockserv) > 0 {
		exporter, err = exportedservice.NewExporter(lockserv, lockboot)
		if err != nil {
			log.Fatal("doozer.DialUri ", lockserv, " (",
				lockboot, "): ", err)
		}

		err = exporter.ListenAndServeNamedHTTP("helloweb", bindto, nil)
		if err != nil {
			log.Fatal("ListenAndServe: ", err)
		}
	} else {
		err = http.ListenAndServe(bindto, nil)
		if err != nil {
			log.Fatal("ListenAndServe: ", err)
		}
	}
}
