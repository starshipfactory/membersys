// Accept the membership request from the member with the given ID.
function acceptMember(id) {
	new $.ajax({
		url: '/admin/api/accept',
		data: {
			uuid: id
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

// Deletes the request from the member with the given ID from the data
// store. This should only be done after the member has been notified
// directly already of the rejection.
function rejectMember(id) {
	if (!confirm("Der Antragsteller wird hierdurch nicht von der Ablehnung " +
		"informiert! Dies muss bereits im Voraus erfolgen!")) {
		return true;
	}

	new $.ajax({
		url: '/admin/api/reject',
		data: {
			uuid: id
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
