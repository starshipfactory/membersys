/**
 * Form handling and validation script
 *
 * Starship Factory
 *
 */
$(document).ready(function() {
		// check if the username exisis in the backend
	$.mockjax({
		url: "users.action",
		response: function(settings) {
			var user = settings.data.username,
				users = ["asdf", "Peter", "Peter2", "George"];
			this.responseText = "true";
			if ( $.inArray( user, users ) !== -1 ) {
				this.responseText = "false";
			}
		},
		responseTime: 500
	});
	
	/** current edit BEGIN */
	
	$.validator.addMethod("feeSelect", function(value, element, params) {
			/**
			params[0] : id of the radiobutton (fee2)
			element : input id="customFee"
			value : contents of "element"
			
			*/
			if ( this.settings.onfocusout) {
				$(params[1]).unbind(".validate-feeSelect").bind("blur.validate-feeSelect", function() {
					$(element).valid();
				});
			}
			
			if($(params[0]).prop('checked')) {
				return true;
			}
			else if($(params[1]).prop('checked') && value >= 50) {
				return true;
			}
			else if( $(params[2]).prop('checked')) {
				return true;
			}
			//return value === target.prop('checked');
			//return this.optional(element) || value == $(params[0]).value();
	}, $.validator.format("--> Mitgliederbeitrag???"));
	
	/** current edit END */

	// validate request form on keyup and submit
	var validator = $("#membershipRequest").validate({
		rules: {
			"mr[name]": "required",
			"mr[address]": "required",
			"mr[city]": "required",
			"mr[zip]": "required",
			"mr[country]": "required",
			"mr[email]": "required",
			/* "mr[fee]" : {
				feeSelect: ["#fee1","#fee2","#reduction"]
			}, */
			/* "mr[fee]" : "required", */
			/* "mr[customFee]" : {
				required: true,
				digits: {
					// doesn't work, AT ALL
					depends: function(element) {
						return $("#reduction:checked")
					}
				}
						//return $('#fee2:checked')
			}, */
			"mr[customFee]" : {
				feeSelect: ["#fee1","#fee2","#reduction"]
			},
			/* "mr[reduction]" : {
				feeSelect: ["#fee1","#fee2","#reduction"]
			}, */
			"mr[statutes]": "required",
			"mr[rules]": "required",
			"mr[ipay]": "required",
			"mr[gt18]": "required",
			
			"mr[username]": {
				required: false,
				minlength: 2,
				remote: "users.action"
			},
			"mr[password]": {
				required: false,
				minlength: 5
			},
			"mr[passwordConfirm]": {
				required: false,
				minlength: 5,
				equalTo: "#password"
			}
		},
		messages: {
			"mr[name]": "Gib deinen Namen an.",
			"mr[address]": "Dieses Feld muss ausgefüllt sein.",
			"mr[city]": "Dieses Feld muss ausgefüllt sein.",
			"mr[zip]": "Dieses Feld muss ausgefüllt sein.",
			"mr[country]": "Dieses Feld muss ausgefüllt sein.",
			"mr[email]": "Dieses Feld muss ausgefüllt sein.",
			"mr[fee]" : "Dieses Feld muss ausgefüllt sein.",
			"mr[fee]" : "Dieses Feld muss ausgefüllt sein.",
			"mr[customFee]" : {
				required: "Dieses Feld muss ausgefüllt sein.",
				digits: jQuery.format("Der Betrag muss grösser als der Mindestbetrag ({0}) sein, andernfalls musst du Reduktion beantragen.")
			},
			"mr[statutes]": "Die Statuten müssen gelesen und akzeptiert werden.",
			"mr[rules]": "Das Reglement muss gelesen und akzeptiert werden.",
			"mr[ipay]": "Bitte bestätigen.",
			"mr[gt18]": "Bitte bestätigen.",
			"mr[username]": {
				required: "Benutzernamen eingeben",
				minlength: jQuery.format("Bitte mindestens {0} Zeichen verwenden"),
				remote: jQuery.format("{0} wurde bereits verwenet")
			},
			"mr[password]": {
				required: "Bitte ein Passwort angeben",
				rangelength: jQuery.format("Mindestens {0} Zeichen verwenden")
			},
			"mr[passwordConfirm]": {
				required: "Wiederhole das Passwort",
				//minlength: jQuery.format("Enter at least {0} characters"),
				equalTo: "Die Passwörter stimmen nicht überein."
			},
			"mr[email]": {
				required: "Bitte gib eine gültige E-Mail Adresse an.",
				minlength: jQuery.format("Mindestens {0} Zeichen verwenden")
			},
			terms: " "
		},
		// set this class to error-labels to indicate valid fields
		success: function(label) {
			// set &nbsp; as text for IE
			label.html("OK").addClass("checked");
		}
	});
});
