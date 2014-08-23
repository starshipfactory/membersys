// Loads the dialog for uploading the membership agreement.
function openUploadAgreement(id, approval_csrf_token, upload_csrf_token) {
	var agreementIdField = $('#agreementId')[0];
	var agreementCsrfTokenField = $('#agreementCsrfToken')[0];
	var agreementUploadCsrfTokenField = $('#agreementUploadCsrfToken')[0];

	var agreementForm = $('#agreementForm')[0];
	var agreementFile = $('#agreementFile')[0];

	$('#agreementUploadError').alert();
	if (!$('#agreementUploadError').hasClass('hide'))
		$('#agreementUploadError').addClass('hide');
	$('#formUploadModal').modal('show');
	agreementIdField.value = id;
	agreementCsrfTokenField.value = approval_csrf_token;
	agreementUploadCsrfTokenField.value = upload_csrf_token;
}

// Displays the size of the agreement file.
function agreementFileSelected() {
	var agreementFile = $('#agreementFile')[0].files[0];
	var indicator = $('#agreementUploadProgress')[0];

    // Clear all child nodes of the upload indicator.
    while (indicator.childNodes.length > 0)
    	indicator.removeChild(indicator.firstChild);

    if (agreementFile) {
    	if (agreementFile.size > 1.5*1048576) {
          indicator.appendChild(document.createTextNode(
            (Math.round(agreementFile.size * 100 / 1048576) / 100).toString() + ' MB hochzuladen'));
        } else {
          indicator.appendChild(document.createTextNode(
            (Math.round(agreementFile.size * 100 / 1024) / 100).toString() + ' KB hochzuladen'));
        }
    } else {
        indicator.appendChild(document.createTextNode("Keine Datei ausgewählt"));
    }
}

// Actually uploads the file.
function doUploadAgreement() {
	var agreementForm = $('#agreementForm')[0];
	var agreementProgress = $('#agreementUploadProgress')[0];
	var agreementFile = $('#agreementFile')[0];

	var agreementIdField = $('#agreementId')[0];
	var agreementCsrfTokenField = $('#agreementCsrfToken')[0];
	var agreementUploadCsrfTokenField = $('#agreementUploadCsrfToken')[0];

	var agreementBtn = $('#agreementUploadBtn')[0];

	agreementBtn.disabled = "disabled";

	var data = new FormData();

	$.each(agreementFile.files, function(key, value) {
		data.append(key, value);
	});

	data.append('csrf_token', agreementUploadCsrfTokenField.value);
	data.append('uuid', agreementIdField.value);

	$.ajax({
		url: '/admin/api/agreement-upload',
		type: 'POST',
		data: data,
		cache: false,
		dataType: 'json',
		processData: false,  // Don't process the files.
		contentType: false,
		success: function(data, textStatus, jqXHR) {
			if (typeof data.error === 'undefined') {
				acceptMember(agreementIdField.value, agreementCsrfTokenField.value);
			} else {
				var errorText = $('#agreementErrorText')[0];

				while (errorText.childNodes.length > 0)
					errorText.removeChild(errorText.firstChild);

				errorText.appendChild(document.createTextNode(data.error));

				if ($('#agreementUploadError').hasClass('hide'))
					$('#agreementUploadError').removeClass('hide');
			}
			agreementBtn.disabled = null;
		},
		error: function(jqXHR, textStatus, errorThrown) {
			var errorText = $('#agreementErrorText')[0];

			while (errorText.childNodes.length > 0)
				errorText.removeChild(errorText.firstChild);

			errorText.appendChild(document.createTextNode(textStatus + ': ' +
				jqXHR.responseText));

			if ($('#agreementUploadError').hasClass('hide'))
				$('#agreementUploadError').removeClass('hide');

			agreementBtn.disabled = null;
		}
	});
}

// Cancels a queued membership application entry.
function cancelQueued(id, csrf_token) {
	new $.ajax({
		url: '/admin/api/cancel-queued',
		data: {
			uuid: id,
			csrf_token: csrf_token
		},
		type: 'POST',
		success: function(response) {
			var tr = $('#q-' + id);
			var tbodies = tr.parent();
			for (i = 0; i < tbodies.length; i++)
				for (j = 0; j < tbodies[i].childNodes.length; j++)
					if (tbodies[i].childNodes[j].id == 'q-' + id)
						tbodies[i].removeChild(tbodies[i].childNodes[j]);
		}
	});
}

// Removes a member from the organization.
function goodbyeMember(id, csrf_token) {
	$('#reasonUser')[0].value = id;
	$('#reasonCsrfToken')[0].value = csrf_token;
	$('#reasonText')[0].value = '';
	$('#reasonEnterModal').modal('show');
}

function doGoodbyeMember() {
	var id = $('#reasonUser')[0].value;
	var csrf_token = $('#reasonCsrfToken')[0].value;
	var reason = $('#reasonText')[0].value;

	new $.ajax({
		url: '/admin/api/goodbye-member',
		data: {
			id: id,
			csrf_token: csrf_token,
			reason: reason
		},
		type: 'POST',
		success: function(response) {
			var bid = id.replace('@', '_').replace('.', '_');
			var tr = $('#mem-' + bid);
			var tbodies = tr.parent();
			for (i = 0; i < tbodies.length; i++) {
				for (j = 0; j < tbodies[i].childNodes.length; j++) {
					if (tbodies[i].childNodes[j].id == 'mem-' + bid)
						tbodies[i].removeChild(tbodies[i].childNodes[j]);
				}
			}

			$('#reasonUser')[0].value = '';
			$('#reasonCsrfToken')[0].value = '';
			$('#reasonText')[0].value = '';
			$('#reasonEnterModal').modal('hide');
		}
	});
}

// Accepts the membership request from the member with the given ID.
function acceptMember(id, csrf_token) {
	new $.ajax({
		url: '/admin/api/accept',
		data: {
			uuid: id,
			csrf_token: csrf_token
		},
		type: 'POST',
		success: function(response) {
			var tr = $('#' + id);
			var tbodies = tr.parent();
			for (i = 0; i < tbodies.length; i++)
				for (j = 0; j < tbodies[i].childNodes.length; j++)
					if (tbodies[i].childNodes[j].id == id)
						tbodies[i].removeChild(tbodies[i].childNodes[j]);

			$('#formUploadModal').modal('hide');
			$('#agreementCsrfToken')[0].value = '';
			$('#agreementUploadCsrfToken')[0].value = '';
			$('#agreementForm')[0].reset();
		}
	});
	return true;
}

// Deletes the request from the member with the given ID from the data
// store. This should only be done after the member has been notified
// directly already of the rejection.
function rejectMember(id, csrf_token) {
	if (!confirm("Der Antragsteller wird hierdurch nicht von der Ablehnung " +
		"informiert! Dies muss bereits im Voraus erfolgen!")) {
		return true;
	}

	new $.ajax({
		url: '/admin/api/reject',
		data: {
			uuid: id,
			csrf_token: csrf_token
		},
		type: 'POST',
		success: function(response) {
			var tr = $('#' + id);
			var tbodies = tr.parent();
			for (i = 0; i < tbodies.length; i++)
				for (j = 0; j < tbodies[i].childNodes.length; j++)
					if (tbodies[i].childNodes[j].id == id)
						tbodies[i].removeChild(tbodies[i].childNodes[j]);
		}
	});
	return true;
}

// Use AJAX to load a list of all organization members and populate the
// corresponding table.
function loadMembers(start) {
	new $.ajax({
		url: '/admin/api/members',
		data: {
			start: start,
		},
		type: 'GET',
		success: function(response) {
			var body = $('#memberlist tbody')[0];
			var members = response.members;
			var token = response.csrf_token;
			var i = 0;

			while (body.childNodes.length > 0)
				body.removeChild(body.firstChild);

			if (members == null || members.length == 0) {
				var tr = document.createElement('tr');
				var td = document.createElement('td');
				td.colspan = 7;
				td.appendChild(document.createTextNode('Derzeit verfügen wir über keine Mitglieder.'));
				tr.appendChild(td);
				body.appendChild(tr);
				return;
			}

			for (i = 0; i < members.length; i++) {
				var id = members[i].email;
				var bid = id.replace('@', '_').replace('.', '_');
				var tr = document.createElement('tr');
				var td;
				var a;

				tr.id = bid;

				td = document.createElement('td');
				td.appendChild(document.createTextNode(members[i].name));
				tr.appendChild(td);

				td = document.createElement('td');
				td.appendChild(document.createTextNode(members[i].street));
				tr.appendChild(td);

				td = document.createElement('td');
				td.appendChild(document.createTextNode(members[i].city));
				tr.appendChild(td);

				td = document.createElement('td');
				if (members[i].username != null)
					td.appendChild(document.createTextNode(members[i].username));
				else
					td.appendChild(document.createTextNode("none"));
				tr.appendChild(td);

				td = document.createElement('td');
				td.appendChild(document.createTextNode(members[i].email));
				tr.appendChild(td);

				td = document.createElement('td');
				td.appendChild(document.createTextNode(
					members[i].fee + " CHF pro " +
					(members[i].fee_yearly ? "Jahr" : "Monat")
					));
				tr.appendChild(td);

				td = document.createElement('td');
				a = document.createElement('a');
				a.href = "#";
				a.onclick = function(e) {
					var tr = e.srcElement.parentNode.parentNode;
					var email = tr.childNodes[4].firstChild.data;
					goodbyeMember(email, token);
				}
				a.appendChild(document.createTextNode('Verabschieden'));
				td.appendChild(a);
				tr.appendChild(td);

				body.appendChild(tr);
			}
		},
	});

	return true;
}

// Use AJAX to load a list of all membership applications and populate the
// corresponding table.
function loadApplicants(criterion, start) {
	new $.ajax({
		url: '/admin/api/applicants',
		data: {
			start: start,
			criterion: criterion,
		},
		type: 'GET',
		success: function(response) {
			var body = $('#applicantlist tbody')[0];
			var applicants = response.applicants;
			var approval_token = response.approval_csrf_token;
			var rejection_token = response.rejection_csrf_token;
			var upload_token = response.agreement_upload_csrf_token;
			var i = 0;

			while (body.childNodes.length > 0)
				body.removeChild(body.firstChild);

			if (applicants == null || applicants.length == 0) {
				var tr = document.createElement('tr');
				var td = document.createElement('td');
				td.colspan = 5;
				td.appendChild(document.createTextNode('Derzeit sind keine Mitgliedsanträge hängig.'));
				tr.appendChild(td);
				body.appendChild(tr);
				return;
			}

			for (i = 0; i < applicants.length; i++) {
				var applicant = applicants[i];
				var tr = document.createElement('tr');
				var td;
				var a;

				tr.id = applicant.key;

				td = document.createElement('td');
				td.appendChild(document.createTextNode(applicant.name));
				tr.appendChild(td);

				td = document.createElement('td');
				td.appendChild(document.createTextNode(applicant.street));
				tr.appendChild(td);

				td = document.createElement('td');
				td.appendChild(document.createTextNode(applicant.city));
				tr.appendChild(td);

				td = document.createElement('td');
				td.appendChild(document.createTextNode(
					applicant.fee + " CHF pro " +
					(applicant.fee_yearly ? "Jahr" : "Monat")
					));
				tr.appendChild(td);

				td = document.createElement('td');
				a = document.createElement('a');
				a.href = "#";
				a.onclick = function(e) {
					var tr = e.srcElement.parentNode.parentNode;
					var id = tr.id;
					console.log(id + " upload commencing");
					openUploadAgreement(id, approval_token, upload_token);
				}
				a.appendChild(document.createTextNode('Annehmen'));
				td.appendChild(a);

				td.appendChild(document.createTextNode(' '));

				a = document.createElement('a');
				a.href = "#";
				a.onclick = function(e) {
					var tr = e.srcElement.parentNode.parentNode;
					var id = tr.id;
					rejectMember(id, rejection_token);
				}
				a.appendChild(document.createTextNode('Ablehnen'));
				td.appendChild(a);
				tr.appendChild(td);

				body.appendChild(tr);
			}
		},
	});

	return true;
}

// Register the required functions for switching between the different tabs.
function load() {
	$('a[href="#members"]').on('show.bs.tab', function(e) {
		loadMembers("");
	});

	$('a[href="#applicants"]').on('show.bs.tab', function(e) {
		loadApplicants("", "");
	});

	loadMembers("");

	return true;
}