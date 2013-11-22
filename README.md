membersys
=========

membersys is a web service for applying for membership in an organization,
written for the Starship Factory.


Requirements
------------

membersys is relatively self-sufficient. It can run in a
doozer-anonymous-port setting, but that Doozer dependency is optional except
for compiling. Other than that it can run on any networked host.


Building
--------

For building, you will need to install the doozer-exportedservice Go package
first. Under Debian GNU/Linux, this can be done by running the following
command as root:

	% apt-get install golang-doozer-exportedservice-dev

Under other systems, you can copy the source files to
${GOPATH}/src/ancient-solutions.com/doozer/exportedservice.

Once this is done, run

	% go build

in order to get a working binary. It will be called membersys and will be
statically linked, so you can just use it anywhere.


Installing
----------

For installing, you will have to copy the form.html and printlayout.html
files as well as the css and js directory into the destination template
directory, such as /usr/local/share/membersys. Then, copy the
binary you built previously to the destination binary directory, e.g.
/usr/local/bin.

Then you can run the binary and pass it the --template-dir flag, pointing
to the directory where you installed the templates, e.g.

	% /usr/local/bin/membersys --template-dir=/usr/local/share/membersys


Running
-------

If you want to use membersys productively, it is recommended that you use
a separate program to launch it and redirect all output to logs. Currently,
membersys doesn't daemonize, so it will always run in the foreground. Using
a tool like run-as-daemon will work around this easily.


Monitoring
----------

Like any good Go program, membersys exports a few variables under
/debug/vars:

* num-http-requests: the total number of HTTP requests received by the
  binary, including requests for favicon.ico, CSS and JS files, etc.
* num-successful-form-submissions: number of times the request form has
  been filled out correctly and submitted.
* num-form-submission-errors: maps by error type the different reasons why
  requests have been rejected (e.g. no name was specified), and how many
  requests of the type have been rejected.


Roadmap
-------

For future releases, we are planning to add the following features:

For release 0.2, we plan to store all successfully submitted requests in an
Apache Cassandra database. The print form will get a barcode with the row
ID of the record, making the record easier to find using a barcode scanner.

Release 0.3 will get mail verification functionality. Applicants will
receive an e-mail to the address they provided, and requests will only be
considered when a link given in the e-mail has been followed.

Release 1.0 will get administrative functionality for authenticated users.
If they are identified to be in a given scope, they will be able to accept
or reject applicants from the web interface. Applicants can either be
searched for by given criteria or looked up directly by their row ID,
which can be scanned from the printed form using a barcode scanner.
