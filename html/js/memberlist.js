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

// Retrieve and display detailed information about a specific member.
function loadMember(email) {
	new $.ajax({
		url: '/admin/api/member',
		data: {
			email: email,
		},
		type: 'GET',
		success: function(response) {
			var label = $('#memberDetailLabel')[0];
			var data = $('#memberDetailData')[0];
			var md = response["member_data"];
			var dt;
			var row;
			var col;
			var inner_el;
			var abbr;

			while (label.childNodes.length > 0)
				label.removeChild(label.firstChild);

			label.appendChild(document.createTextNode(md.name));

			while (data.childNodes.length > 0)
				data.removeChild(data.firstChild);

			row = document.createElement('div');
			row.className = 'row';

			col = document.createElement('div');
			col.className = 'col-xs-4';
			inner_el = document.createElement('strong');
			inner_el.appendChild(document.createTextNode('Adresse'));
			col.appendChild(inner_el);
			row.appendChild(col);

			col = document.createElement('div');
			col.className = 'col-xs-8';
			inner_el = document.createElement('address');
			inner_el.appendChild(document.createTextNode(md.street));
			inner_el.appendChild(document.createElement('br'));
			inner_el.appendChild(document.createTextNode(md.zipcode));
			inner_el.appendChild(document.createTextNode(' '));
			inner_el.appendChild(document.createTextNode(md.city));
			inner_el.appendChild(document.createElement('br'));
			inner_el.appendChild(document.createTextNode(md.country));
			inner_el.appendChild(document.createElement('br'));

			a = document.createElement('a');
			a.href = '#';
			a.onclick = function() {
				$('#memberDetailModal').modal('hide');
				editMemberAddress(md.email, md.name, md.street,
					md.zipcode, md.city, md.country);
			}
			a.appendChild(document.createTextNode('Bearbeiten'));
			inner_el.appendChild(a);
			inner_el.appendChild(document.createElement('br'));

			if (md.phone != null) {
				abbr = document.createElement('abbr');
				inner_el.appendChild(document.createElement('br'));
				abbr.title = 'Telephon';
				abbr.appendChild(document.createTextNode('T:'));
				inner_el.appendChild(abbr);
				inner_el.appendChild(document.createTextNode(' ' +
					md.phone));

				abbr.ondblclick = function() {
					$('#memberDetailModal').modal('hide');
					editMemberPhone(md.email, md.name, md.phone);
				}
			} else {
				a = document.createElement('a');
				a.href = '#';
				a.onclick = function() {
					$('#memberDetailModal').modal('hide');
					editMemberPhone(md.email, md.name, '');
				}
				a.appendChild(document.createTextNode('Telephon eintragen'));
				inner_el.appendChild(a);
				inner_el.appendChild(document.createElement('br'));
			}

			abbr = document.createElement('abbr');
			inner_el.appendChild(document.createElement('br'));
			abbr.title = 'Email';
			abbr.appendChild(document.createTextNode('E:'));
			inner_el.appendChild(abbr);

			inner_el.appendChild(document.createTextNode(' '));

			abbr = document.createElement('a');
			abbr.href = "mailto:" + md.email;
			abbr.appendChild(document.createTextNode(md.email));
			inner_el.appendChild(abbr);

			col.appendChild(inner_el);
			row.appendChild(col);

			data.appendChild(row);

			row = document.createElement('div');
			row.className = 'row';

			col = document.createElement('div');
			col.className = 'col-xs-4';
			inner_el = document.createElement('strong');
			inner_el.appendChild(document.createTextNode('Gebühren'));
			col.appendChild(inner_el);
			row.appendChild(col);

			col = document.createElement('div');
			col.className = 'col-xs-8';

			col.appendChild(document.createTextNode(
				md.fee + " CHF pro " + (md.fee_yearly ? "Jahr" : "Monat")));
			col.appendChild(document.createTextNode(' '));

			inner_el = document.createElement('a');
			inner_el.href = "#";
			inner_el.onclick = function() {
				$('#memberDetailModal').modal('hide');
				editMembershipFee(md.email, md.name, md.fee, md.fee_yearly);
			}
			inner_el.appendChild(document.createTextNode('Bearbeiten'));

			col.appendChild(inner_el);
			row.appendChild(col);
			data.appendChild(row);

			row = document.createElement('div');
			row.className = 'row';
			col = document.createElement('div');
			col.className = 'col-xs-4';
			inner_el = document.createElement('strong');
			inner_el.appendChild(document.createTextNode('Schlüssel'));
			col.appendChild(inner_el);
			row.appendChild(col);

			col = document.createElement('div');
			col.className = 'col-xs-8';
			inner_el = document.createElement('input');
			inner_el.type = 'checkbox';
			inner_el.checked = md.has_key;
			inner_el.id = 'memberDetailHasKey';
			inner_el.onchange = function() {
				keyElem = $('#memberDetailHasKey')[0];
				editHasKey(md.email, keyElem.checked);
			}
			col.appendChild(inner_el);
			inner_el = document.createElement('label');
			inner_el.appendChild(document.createTextNode(
				'Mitglied verfügt über einen Schlüssel'));
			inner_el.for = 'memberDetailHasKey';
			col.appendChild(inner_el);
			row.appendChild(col);
			data.appendChild(row)

			row = document.createElement('div');
			row.className = 'row';
			col = document.createElement('div');
			col.className = 'col-xs-4';
			inner_el = document.createElement('strong');
			inner_el.appendChild(document.createTextNode('Gezahlt bis'));
			col.appendChild(inner_el);
			row.appendChild(col);

			col = document.createElement('div');
			col.className = 'col-xs-8';
			inner_el = document.createElement('input');
			inner_el.type = 'date';
			inner_el.id = 'memberDetailPaymentsTo';
			if (md.payments_caught_up_to != null &&
				md.payments_caught_up_to > 0) {
				dt = new Date(value=md.payments_caught_up_to * 1000);
				inner_el.value = dt.toISOString().split('T')[0];
			}
			inner_el.onchange = function() {
				dateField = $('#memberDetailPaymentsTo')[0];
				dt = new Date(dateField.value);
				editPaymentsCaughtUpTo(md.email, dt);
			}
			col.appendChild(inner_el);
			row.appendChild(col);
			data.appendChild(row)

			if (md.username != null) {
				row = document.createElement('div');
				row.className = 'row';

				col = document.createElement('div');
				col.className = 'col-xs-4';
				inner_el = document.createElement('strong');
				inner_el.appendChild(document.createTextNode('Benutzerkonto'));
				col.appendChild(inner_el);
				row.appendChild(col);

				col = document.createElement('div');
				col.className = 'col-xs-8';

				col.appendChild(document.createTextNode(md.username));

				row.appendChild(col);
				data.appendChild(row);
			} else {
				row = document.createElement('div');
				row.className = 'row';

				col = document.createElement('div');
				col.className = 'col-xs-12';
				inner_el = document.createElement('a');
				inner_el.href = '#';
				inner_el.onclick = function() {
					$('#memberDetailModal').modal('hide');
					editMemberUser(md.email, md.name, '');
				}
				inner_el.appendChild(document.createTextNode('Benutzernamen setzen'));

				col.appendChild(inner_el);
				row.appendChild(col);
				data.appendChild(row);
			}

			row = document.createElement('div');
			row.className = 'row';

			col = document.createElement('div');
			col.className = 'col-xs-4';
			inner_el = document.createElement('strong');
			inner_el.appendChild(document.createTextNode('Mitgliedschaftsanfrage'));
			col.appendChild(inner_el);
			row.appendChild(col);

			col = document.createElement('div');
			col.className = 'col-xs-8';

			dt = new Date(response.metadata.request_timestamp * 1000);
			col.appendChild(document.createTextNode(dt.toLocaleString()));

			row.appendChild(col);
			data.appendChild(row);

			row = document.createElement('div');
			row.className = 'row';

			col = document.createElement('div');
			col.className = 'col-xs-4';
			inner_el = document.createElement('strong');
			inner_el.appendChild(document.createTextNode('Datum der Mitgliedschaft'));
			col.appendChild(inner_el);
			row.appendChild(col);

			col = document.createElement('div');
			col.className = 'col-xs-8';

			dt = new Date(response.metadata.approval_timestamp * 1000);
			col.appendChild(document.createTextNode(dt.toLocaleString() +
				" von "));

			inner_el = document.createElement('i');
			inner_el.appendChild(document.createTextNode(response.metadata.approver_uid));
			col.appendChild(inner_el);

			row.appendChild(col);
			data.appendChild(row);

			$('#memberDetailModal').modal('show');
		}
	});
	return true;
}

// Edit the membership fee details of the given member.
function editMembershipFee(email, name, fee, fee_yearly) {
	var lbl = $('#memberFeeEditLabel')[0];
	var feef = $('#memberFeeField')[0];
	var who = $('#memberFeeMail')[0];
	var monthly = $('#memberFeeIntervalMonthly')[0];
	var yearly = $('#memberFeeIntervalYearly')[0];

	while (lbl.childNodes.length > 0)
		lbl.removeChild(lbl.firstChild);

	lbl.appendChild(document.createTextNode(name + ": Beitrag bearbeiten"));
	feef.value = fee;
	who.value = email;

	monthly.checked = !fee_yearly;
	yearly.checked = fee_yearly;

	$('#memberFeeEditModal').modal('show');
}

// Update the membership fee of the affected member.
function doEditMembershipFee() {
	var feef = $('#memberFeeField')[0];
	var who = $('#memberFeeMail')[0];
	var monthly = $('#memberFeeIntervalMonthly')[0];
	var yearly = $('#memberFeeIntervalYearly')[0];

	new $.ajax({
		url: '/admin/api/editfee',
		data: {
			email: who.value,
			fee: feef.value,
			fee_yearly: yearly.checked,
		},
		type: 'POST',
		success: function(response) {
			$('#memberFeeEditModal').modal('hide');
			loadMembers(member_offset);
		}
	});
}

// Edit address details of the specified member.
function editMemberAddress(email, name, street, zip, city, country) {
	var lbl = $('#memberAddressEditLabel')[0];
	var streetf = $('#memberAddressStreetField')[0];
	var streetorig = $('#memberAddressStreetOrig')[0];
	var zipcodef = $('#memberAddressZipcodeField')[0];
	var zipcodeorig = $('#memberAddressZipcodeOrig')[0];
	var cityf = $('#memberAddressCityField')[0];
	var cityorig = $('#memberAddressCityOrig')[0];
	var countryf = $('#memberAddressCountryField')[0];
	var countryorig = $('#memberAddressCountryOrig')[0];
	var who = $('#memberAddressMail')[0];

	while (lbl.childNodes.length > 0)
		lbl.removeChild(lbl.firstChild);

	lbl.appendChild(document.createTextNode(name + ': Adresse bearbeiten'));

	streetf.value = street;
	streetorig.value = street;
	zipcodef.value = zip;
	zipcodeorig.value = zip;
	cityf.value = city;
	cityorig.value = city;
	countryf.value = country;
	countryorig.value = country;
	who.value = email;

	$('#memberAddressEditModal').modal('show');
}

// Update address details of the affected member.
function doEditMemberAddress() {
	var streetf = $('#memberAddressStreetField')[0];
	var streetorig = $('#memberAddressStreetOrig')[0];
	var zipcodef = $('#memberAddressZipcodeField')[0];
	var zipcodeorig = $('#memberAddressZipcodeOrig')[0];
	var cityf = $('#memberAddressCityField')[0];
	var cityorig = $('#memberAddressCityOrig')[0];
	var countryf = $('#memberAddressCountryField')[0];
	var countryorig = $('#memberAddressCountryOrig')[0];
	var who = $('#memberAddressMail')[0];

	var origValues = {};
	var newValues = {};

	origValues['street'] = streetorig.value;
	origValues['zipcode'] = zipcodeorig.value;
	origValues['city'] = cityorig.value;
	origValues['country'] = countryorig.value;
	newValues['street'] = streetf.value;
	newValues['zipcode'] = zipcodef.value;
	newValues['city'] = cityf.value;
	newValues['country'] = countryf.value;

	for (var property in origValues) {
		if (newValues[property] != '' &&
			origValues[property] != newValues[property]) {
			new $.ajax({
				url: '/admin/api/edittext',
				data: {
					email: who.value,
					field: property,
					value: newValues[property],
				},
				type: 'POST',
				success: function(response) {
					$('#memberAddressEditModal').modal('hide');
					loadMembers(member_offset);
				}
			});
		}
	}
}

// Edit the phone number of the specified user.
function editMemberPhone(email, name, phone) {
	var lbl = $('#memberPhoneEditLabel')[0];
	var phonef = $('#memberPhoneNumberField')[0];
	var who = $('#memberPhoneMail')[0];

	while (lbl.childNodes.length > 0)
		lbl.removeChild(lbl.firstChild);

	lbl.appendChild(document.createTextNode(name + ': Telephonnummer bearbeiten'));
	phonef.value = phone;
	who.value = email;

	$('#memberPhoneEditModal').modal('show');
}

// Update the stored user name of the affected member.
function doEditMemberPhone() {
	var phonef = $('#memberPhoneNumberField')[0];
	var who = $('#memberPhoneMail')[0];

	new $.ajax({
		url: '/admin/api/edittext',
		data: {
			email: who.value,
			field: 'phone',
			value: phonef.value,
		},
		type: 'POST',
		success: function(response) {
			$('#memberPhoneEditModal').modal('hide');
			loadMembers(member_offset);
		}
	});
}

// Set whether the member has a key.
function editHasKey(email, has_key) {
	new $.ajax({
		url: '/admin/api/editbool',
		data: {
			email: email,
			field: 'has_key',
			value: has_key,
		},
		type: 'POST',
		error: function(jqXHR, textStatus, errorThrown) {
			$('#memberDetailHasKey').popover({
				'title': 'Fehler beim Speichern',
				'content': textStatus,
			});
			$('#memberDetailHasKey').popover('show');
		}
	});
}

// Set the date up to which payments are caught up.
function editPaymentsCaughtUpTo(email, dt) {
	new $.ajax({
		url: '/admin/api/editlong',
		data: {
			email: email,
			field: 'payments_caught_up_to',
			value: Number(dt) / 1000,
		},
		type: 'POST',
		error: function(jqXHR, textStatus, errorThrown) {
			$('#memberDetailPaymentsTo').popover({
				'title': 'Fehler beim Speichern',
				'content': textStatus,
			});
			$('#memberDetailPaymentsTo').popover('show');
		}
	});
}

// Edit the stored user name of the specified user.
function editMemberUser(email, name, username) {
	var lbl = $('#memberUserEditLabel')[0];
	var userf = $('#memberUserField')[0];
	var who = $('#memberUserMail')[0];

	while (lbl.childNodes.length > 0)
		lbl.removeChild(lbl.firstChild);

	lbl.appendChild(document.createTextNode(name + ': Benutzernamen bearbeiten'));
	userf.value = username;
	who.value = email;

	$('#memberUserEditModal').modal('show');
}

// Update the stored user name of the affected member.
function doEditMemberUser() {
	var userf = $('#memberUserField')[0];
	var who = $('#memberUserMail')[0];

	new $.ajax({
		url: '/admin/api/edittext',
		data: {
			email: who.value,
			field: 'username',
			value: userf.value,
		},
		type: 'POST',
		success: function(response) {
			$('#memberUserEditModal').modal('hide');
			loadMembers(member_offset);
		}
	});
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
			var prevarr = $('#members ul.pager li.previous');
			var nextarr = $('#members ul.pager li.next');
			var members = response.members;
			var token = response.csrf_token;
			var i = 0;

			while (body.childNodes.length > 0)
				body.removeChild(body.firstChild);

			if (members == null || members.length == 0) {
				var tr = document.createElement('tr');
				var td = document.createElement('td');
				td.colspan = 7;
				td.appendChild(document.createTextNode('Es wurden keine weiteren Mitglieder gefunden.'));
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

				tr.id = "mem-" + bid;

				td = document.createElement('td');
				td.appendChild(document.createTextNode(members[i].name));
				tr.appendChild(td);

				td = document.createElement('td');
				td.appendChild(document.createTextNode(members[i].city));
				tr.appendChild(td);

				td = document.createElement('td');
				if (members[i].username != null)
					td.appendChild(document.createTextNode(members[i].username));
				else
					td.appendChild(document.createTextNode("Keiner"));
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
				td.appendChild(document.createTextNode(
					members[i].has_key ? 'Ja' : 'Nein'));
				tr.appendChild(td);

				td = document.createElement('td');
				td.appendChild(document.createTextNode(
					members[i].payments_caught_up_to ?
					(new Date(
						value=members[i].payments_caught_up_to*1000))
					.toDateString() : '-'));
				tr.appendChild(td);

				td = document.createElement('td');
				a = document.createElement('a');
				a.href = "#";
				a.onclick = function(e) {
					var target = e.target == null ? e.srcElement : e.target;
					var tr = target.parentNode.parentNode;
					var email = tr.childNodes[3].firstChild.data;
					goodbyeMember(email, token);
				}
				a.appendChild(document.createTextNode('Verabschieden'));
				td.appendChild(a);

				td.appendChild(document.createTextNode(' '));

				a = document.createElement('a');
				a.href = "#";
				a.onclick = function(e) {
					var target = e.target == null ? e.srcElement : e.target;
					var tr = target.parentNode.parentNode;
					var email = tr.childNodes[3].firstChild.data;
					loadMember(email);
				}
				a.appendChild(document.createTextNode('Details'));
				td.appendChild(a);
				tr.appendChild(td);

				body.appendChild(tr);
			}

			if (start.length > 0) {
				prevarr.removeClass('disabled');
			} else {
				prevarr.addClass('disabled');
			}

			if (members.length == page_size) {
				nextarr.removeClass('disabled');
			} else {
				nextarr.addClass('disabled');
			}
		},
	});

	return true;
}

// Go to the next batch of members starting with the current one.
function forwardMembers() {
	var membertable = $('#memberlist tbody tr');
	var lastrecord = membertable[membertable.length - 1];
	var lastid = lastrecord.id.substr(4);

	loadMembers(lastid);
	member_offset = lastid;
}

// Use AJAX to load a list of all membership applications and populate the
// corresponding table.
function loadApplicants(criterion, start, single) {
	new $.ajax({
		url: '/admin/api/applicants',
		data: {
			start: start,
			criterion: criterion,
			single: single,
		},
		type: 'GET',
		success: function(response) {
			var body = $('#applicantlist tbody')[0];
			var prevarr = $('#applicants ul.pager li.previous');
			var nextarr = $('#applicants ul.pager li.next');
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
				var applicantMD = applicants[i].metadata;
				var applicant = applicants[i].member_data;
				var tr = document.createElement('tr');
				var td;
				var a;

				tr.id = applicants[i].key;

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
				dt = new Date(applicantMD.request_timestamp * 1000);
				td.appendChild(document.createTextNode(dt.toLocaleString()));
				br = document.createElement('br');
				td.appendChild(br);
				td.appendChild(document.createTextNode(
					applicantMD.request_source_ip));
				tr.appendChild(td);

				td = document.createElement('td');
				a = document.createElement('a');
				a.href = "#";
				a.onclick = function(e) {
					var target = e.target == null ? e.srcElement : e.target;
					var tr = target.parentNode.parentNode;
					var id = tr.id;
					openUploadAgreement(id, approval_token, upload_token);
				}
				a.appendChild(document.createTextNode('Annehmen'));
				td.appendChild(a);

				td.appendChild(document.createTextNode(' '));

				a = document.createElement('a');
				a.href = "#";
				a.onclick = function(e) {
					var target = e.target == null ? e.srcElement : e.target;
					var tr = target.parentNode.parentNode;
					var id = tr.id.replace("q-", "");
					rejectMember(id, rejection_token);
				}
				a.appendChild(document.createTextNode('Ablehnen'));
				td.appendChild(a);
				tr.appendChild(td);

				body.appendChild(tr);
			}

			if (start.length > 0) {
				prevarr.removeClass('disabled');
			} else {
				prevarr.addClass('disabled');
			}

			if (applicants.length == page_size) {
				nextarr.removeClass('disabled');
			} else {
				nextarr.addClass('disabled');
			}
		},
	});

	return true;
}

// Go to the next batch of members starting with the current one.
function forwardApplicants() {
	var membertable = $('#applicantlist tbody tr');
	var lastrecord = membertable[membertable.length - 1];
	var lastid = lastrecord.id;

	loadApplicants("", lastid);
}

// Use AJAX to load a list of all applicants queued to become organization
// members and populate the corresponding table.
function loadQueue(start) {
	new $.ajax({
		url: '/admin/api/queue',
		data: {
			start: start,
		},
		type: 'GET',
		success: function(response) {
			var body = $('#queuelist tbody')[0];
			var prevarr = $('#queue ul.pager li.previous');
			var nextarr = $('#queue ul.pager li.next');
			var members = response.queued;
			var token = response.csrf_token;
			var i = 0;

			while (body.childNodes.length > 0)
				body.removeChild(body.firstChild);

			if (members == null || members.length == 0) {
				var tr = document.createElement('tr');
				var td = document.createElement('td');
				td.colspan = 7;
				td.appendChild(document.createTextNode(
					'Derzeit sind keine Mitgliedsanträge in Verarbeitung.'));
				tr.appendChild(td);
				body.appendChild(tr);
				return;
			}

			for (i = 0; i < members.length; i++) {
				var member = members[i];
				var tr = document.createElement('tr');
				var td;
				var a;

				tr.id = "q-" + member.key;

				td = document.createElement('td');
				td.appendChild(document.createTextNode(member.name));
				tr.appendChild(td);

				td = document.createElement('td');
				td.appendChild(document.createTextNode(member.street));
				tr.appendChild(td);

				td = document.createElement('td');
				td.appendChild(document.createTextNode(member.city));
				tr.appendChild(td);

				td = document.createElement('td');
				if (members[i].username != null)
					td.appendChild(document.createTextNode(member.username));
				else
					td.appendChild(document.createTextNode("Keiner"));
				tr.appendChild(td);

				td = document.createElement('td');
				td.appendChild(document.createTextNode(member.email));
				tr.appendChild(td);

				td = document.createElement('td');
				td.appendChild(document.createTextNode(
					member.fee + " CHF pro " +
					(member.fee_yearly ? "Jahr" : "Monat")
					));
				tr.appendChild(td);

				td = document.createElement('td');
				a = document.createElement('a');
				a.href = "#";
				a.onclick = function(e) {
					var target = e.target == null ? e.srcElement : e.target;
					var tr = target.parentNode.parentNode;
					cancelQueued(tr.id.replace("q-", ""), token);
				}
				a.appendChild(document.createTextNode('Abbrechen'));
				td.appendChild(a);
				tr.appendChild(td);

				body.appendChild(tr);
			}

			if (start.length > 0) {
				prevarr.removeClass('disabled');
			} else {
				prevarr.addClass('disabled');
			}

			if (members.length == page_size) {
				nextarr.removeClass('disabled');
			} else {
				nextarr.addClass('disabled');
			}
		},
	});

	return true;
}

// Go to the next batch of queued records starting with the current one.
function forwardQueue() {
	var membertable = $('#queuelist tbody tr');
	var lastrecord = membertable[membertable.length - 1];
	var lastid = lastrecord.id.substr(2);

	loadQueue(lastid);
}

// Use AJAX to load a list of all applicants queued to become organization
// members and populate the corresponding table.
function loadDequeue(start) {
	new $.ajax({
		url: '/admin/api/dequeue',
		data: {
			start: start,
		},
		type: 'GET',
		success: function(response) {
			var body = $('#dequeuelist tbody')[0];
			var prevarr = $('#dequeue ul.pager li.previous');
			var nextarr = $('#dequeue ul.pager li.next');
			var members = response.queued;
			var token = response.csrf_token;
			var i = 0;

			while (body.childNodes.length > 0)
				body.removeChild(body.firstChild);

			if (members == null || members.length == 0) {
				var tr = document.createElement('tr');
				var td = document.createElement('td');
				td.colspan = 4;
				td.appendChild(document.createTextNode(
					'Derzeit sind keine Löschungen in Verarbeitung.'));
				tr.appendChild(td);
				body.appendChild(tr);
				return;
			}

			for (i = 0; i < members.length; i++) {
				var member = members[i];
				var tr = document.createElement('tr');
				var td;
				var a;

				tr.id = "dq-" + member.key;

				td = document.createElement('td');
				td.appendChild(document.createTextNode(member.name));
				tr.appendChild(td);

				td = document.createElement('td');
				td.appendChild(document.createTextNode(member.street));
				tr.appendChild(td);

				td = document.createElement('td');
				td.appendChild(document.createTextNode(member.city));
				tr.appendChild(td);

				td = document.createElement('td');
				if (members[i].username != null)
					td.appendChild(document.createTextNode(member.username));
				else
					td.appendChild(document.createTextNode("Keiner"));
				tr.appendChild(td);

				td = document.createElement('td');
				td.appendChild(document.createTextNode(member.email));
				tr.appendChild(td);

				td = document.createElement('td');
				td.appendChild(document.createTextNode(
					member.fee + " CHF pro " +
					(member.fee_yearly ? "Jahr" : "Monat")
					));
				tr.appendChild(td);

				body.appendChild(tr);
			}

			if (start.length > 0) {
				prevarr.removeClass('disabled');
			} else {
				prevarr.addClass('disabled');
			}

			if (members.length == page_size) {
				nextarr.removeClass('disabled');
			} else {
				nextarr.addClass('disabled');
			}
		},
	});

	return true;
}

// Go to the next batch of queued records starting with the current one.
function forwardDequeue() {
	var membertable = $('#dequeuelist tbody tr');
	var lastrecord = membertable[membertable.length - 1];
	var lastid = lastrecord.id.substr(3);

	loadQueue(lastid);
}

// Use AJAX to load a list of all members in the trash and populate the
// corresponding table.
function loadTrash(start) {
	new $.ajax({
		url: '/admin/api/trash',
		data: {
			start: start,
		},
		type: 'GET',
		success: function(response) {
			var body = $('#trashlist tbody')[0];
			var prevarr = $('#trash ul.pager li.previous');
			var nextarr = $('#trash ul.pager li.next');
			var i = 0;

			while (body.childNodes.length > 0)
				body.removeChild(body.firstChild);

			if (response == null || response.length == 0) {
				var tr = document.createElement('tr');
				var td = document.createElement('td');
				td.colspan = 4;
				td.appendChild(document.createTextNode(
					'Derzeit sind keine Löschungen in Verarbeitung.'));
				tr.appendChild(td);
				body.appendChild(tr);
				return;
			}

			for (i = 0; i < response.length; i++) {
				var member = response[i];
				var tr = document.createElement('tr');
				var td;
				var a;

				tr.id = "dq-" + member.key;

				td = document.createElement('td');
				td.appendChild(document.createTextNode(member.name));
				tr.appendChild(td);

				td = document.createElement('td');
				td.appendChild(document.createTextNode(member.street));
				tr.appendChild(td);

				td = document.createElement('td');
				td.appendChild(document.createTextNode(member.city));
				tr.appendChild(td);

				td = document.createElement('td');
				if (member.username != null)
					td.appendChild(document.createTextNode(member.username));
				else
					td.appendChild(document.createTextNode("Keiner"));
				tr.appendChild(td);

				td = document.createElement('td');
				td.appendChild(document.createTextNode(member.email));
				tr.appendChild(td);

				td = document.createElement('td');
				td.appendChild(document.createTextNode(
					member.fee + " CHF pro " +
					(member.fee_yearly ? "Jahr" : "Monat")
					));
				tr.appendChild(td);

				body.appendChild(tr);
			}

			if (start.length > 0) {
				prevarr.removeClass('disabled');
			} else {
				prevarr.addClass('disabled');
			}

			if (response.length == page_size) {
				nextarr.removeClass('disabled');
			} else {
				nextarr.addClass('disabled');
			}
		},
	});

	return true;
}

// Go to the next batch of queued records starting with the current one.
function forwardTrash() {
	var membertable = $('#trashlist tbody tr');
	var lastrecord = membertable[membertable.length - 1];
	var lastid = lastrecord.id.substr(3);

	loadTrash(lastid);
}

// Register the required functions for switching between the different tabs.
function load() {
	$('a[href="#members"]').on('show.bs.tab', function(e) {
		loadMembers("");
	});

	$('a[href="#applicants"]').on('show.bs.tab', function(e) {
		loadApplicants("", "");
	});

	$('a[href="#queue"]').on('show.bs.tab', function(e) {
		loadQueue("");
	});

	$('a[href="#dequeue"]').on('show.bs.tab', function(e) {
		loadDequeue("");
	});

	$('a[href="#trash"]').on('show.bs.tab', function(e) {
		loadTrash("");
	});

	loadMembers("");

	return true;
}
