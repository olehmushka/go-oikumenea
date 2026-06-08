package application

import (
	"strings"

	"github.com/nyaruka/phonenumbers"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
)

// normalizeEmail lowercases and trims a contact email before validation/storage. The column is
// citext, so case-insensitivity is also enforced at the DB; normalizing here keeps the stored form
// canonical and the derived provider stable (D-PersonContactChannels).
func normalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// emailProviders maps a known email domain to a stable provider code. Derived on write and stored in
// person_emails.provider; "" when the domain is not in the map (D-PersonContactChannels). Closed
// vocabulary by design — extend here, not via operator config.
var emailProviders = map[string]string{
	"gmail.com":      "google",
	"googlemail.com": "google",
	"outlook.com":    "microsoft",
	"hotmail.com":    "microsoft",
	"live.com":       "microsoft",
	"msn.com":        "microsoft",
	"yahoo.com":      "yahoo",
	"ymail.com":      "yahoo",
	"proton.me":      "proton",
	"protonmail.com": "proton",
	"icloud.com":     "apple",
	"me.com":         "apple",
	"mac.com":        "apple",
	"gmx.com":        "gmx",
	"gmx.net":        "gmx",
	"ukr.net":        "ukrnet",
	"i.ua":           "iua",
	"meta.ua":        "metaua",
}

// emailProvider derives the provider code from a (normalized) email address's domain; "" when there
// is no @ or the domain is unknown.
func emailProvider(address string) string {
	at := strings.LastIndexByte(address, '@')
	if at < 0 {
		return ""
	}
	return emailProviders[address[at+1:]]
}

// normalizeHandle trims a social handle and drops a single leading '@' so the stored form is canonical
// (D-PersonSocialChannels). Case is preserved (the active-handle uniqueness index lowercases).
func normalizeHandle(s string) string {
	return strings.TrimPrefix(strings.TrimSpace(s), "@")
}

// profileURLTemplates maps a platform code to a handle->URL template ("%s" = the bare handle). Used to
// derive person_social_accounts.profile_url on write when the caller supplies no explicit URL; "" (no
// template) leaves the URL unset. Closed vocabulary by design — extend here, not via operator config.
var profileURLTemplates = map[string]string{
	"telegram":  "https://t.me/%s",
	"instagram": "https://instagram.com/%s",
	"linkedin":  "https://www.linkedin.com/in/%s",
	"x":         "https://x.com/%s",
	"facebook":  "https://facebook.com/%s",
}

// deriveProfileURL builds a profile URL from the platform + handle when there is a known template and
// the handle is non-empty; otherwise "" (the column stays NULL / the caller's value is kept upstream).
func deriveProfileURL(platformCode, handle string) string {
	tmpl, ok := profileURLTemplates[platformCode]
	if !ok || handle == "" {
		return ""
	}
	return strings.Replace(tmpl, "%s", handle, 1)
}

// normalizePhone parses a raw phone number to E.164 and derives its ISO-3166-1 alpha-2 country
// (D-PersonContactChannels), using github.com/nyaruka/phonenumbers (libphonenumber). The input is
// expected in international form (leading +); a number that cannot be parsed or is not a valid number
// yields domain.ErrUnparseablePhone. The derived country is "" when the library cannot resolve a
// region (it then maps to a NULL country column).
func normalizePhone(raw string) (e164, country string, err error) {
	// "ZZ" = unknown default region: a leading "+" supplies the country calling code.
	num, perr := phonenumbers.Parse(strings.TrimSpace(raw), "ZZ")
	if perr != nil || !phonenumbers.IsValidNumber(num) {
		return "", "", domain.ErrUnparseablePhone
	}
	e164 = phonenumbers.Format(num, phonenumbers.E164)
	region := phonenumbers.GetRegionCodeForNumber(num)
	if region == "" || region == "ZZ" {
		return e164, "", nil
	}
	return e164, region, nil
}
