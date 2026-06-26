package main

// placeholders maps Presidio entity types to the redaction token used to
// replace detected spans.
var placeholders = map[string]string{
	"PERSON":        "[PERSON]",
	"LOCATION":      "[LOCATION]",
	"GPE":           "[LOCATION]",
	"ORGANIZATION":  "[ORG]",
	"PHONE_NUMBER":  "[PHONE]",
	"EMAIL_ADDRESS": "[EMAIL]",
	"CREDIT_CARD":   "[CREDIT_CARD]",
	"IP_ADDRESS":    "[IP]",
	"URL":           "[URL]",
	"IBAN_CODE":     "[IBAN]",
	"US_SSN":        "[SSN]",
	"DATE_TIME":     "[DATE]",
	"NRP":           "[GROUP]",
}

// placeholderFor returns the redaction token for the given Presidio entity
// type. Unknown entity types are wrapped in brackets verbatim, e.g. an unknown
// type "MEDICAL_LICENSE" becomes "[MEDICAL_LICENSE]".
func placeholderFor(entityType string) string {
	if p, ok := placeholders[entityType]; ok {
		return p
	}
	return "[" + entityType + "]"
}
