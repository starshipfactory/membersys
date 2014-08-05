// Load the dialog for uploading the membership agreement.
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

// Display the size of the agreement file.
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

// Actually upload the file.
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

// Accept the membership request from the member with the given ID.
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
