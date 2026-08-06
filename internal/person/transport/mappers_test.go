// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	personapi "github.com/olehmushka/go-oikumenea/internal/conjure/oikumenea/person"
	"github.com/olehmushka/go-oikumenea/internal/person/domain"
)

// mapErrorContract pins the HTTP classification of every person domain error `mapError` is expected to
// translate: each sentinel must map to a typed Conjure error (PersonInvalid=400 / PersonConflict=409 /
// PersonNotFound=404 / PersonLifecycleConflict=409), never the `default:` arm that wraps to a generic
// 500. This is the regression guard for the class of bug where a catalog/FK/child sentinel is returned
// but missing from the switch (e.g. ErrUnknownLegalBasis / ErrUnknownEthnicityType, which used to 500).
var mapErrorContract = []struct {
	name string
	err  error
	want func(error) bool
}{
	// ---- catalog / FK / bad-reference inputs -> 400 PersonInvalid ----
	{"ErrUnknownRank", domain.ErrUnknownRank, personapi.IsPersonInvalid},
	{"ErrUnknownCountry", domain.ErrUnknownCountry, personapi.IsPersonInvalid},
	{"ErrUnknownLocale", domain.ErrUnknownLocale, personapi.IsPersonInvalid},
	{"ErrUnknownContactType", domain.ErrUnknownContactType, personapi.IsPersonInvalid},
	{"ErrUnknownPlatform", domain.ErrUnknownPlatform, personapi.IsPersonInvalid},
	{"ErrUnknownRelationType", domain.ErrUnknownRelationType, personapi.IsPersonInvalid},
	{"ErrUnknownCounterpart", domain.ErrUnknownCounterpart, personapi.IsPersonInvalid},
	{"ErrUnknownRelationshipKind", domain.ErrUnknownRelationshipKind, personapi.IsPersonInvalid},
	{"ErrUnknownLanguage", domain.ErrUnknownLanguage, personapi.IsPersonInvalid},
	{"ErrUnknownLocation", domain.ErrUnknownLocation, personapi.IsPersonInvalid},
	{"ErrUnknownLegalBasis", domain.ErrUnknownLegalBasis, personapi.IsPersonInvalid},
	{"ErrUnknownEthnicityType", domain.ErrUnknownEthnicityType, personapi.IsPersonInvalid},
	// ---- other invalid-input -> 400 PersonInvalid ----
	{"ErrInvalid", domain.ErrInvalid, personapi.IsPersonInvalid},
	{"ErrRelationCategory", domain.ErrRelationCategory, personapi.IsPersonInvalid},
	{"ErrSelfRelationship", domain.ErrSelfRelationship, personapi.IsPersonInvalid},
	{"ErrPlatformNotMessenger", domain.ErrPlatformNotMessenger, personapi.IsPersonInvalid},
	{"ErrChannelNotOwned", domain.ErrChannelNotOwned, personapi.IsPersonInvalid},
	{"ErrUnparseablePhone", domain.ErrUnparseablePhone, personapi.IsPersonInvalid},
	{"ErrColorMismatch", domain.ErrColorMismatch, personapi.IsPersonInvalid},
	{"ErrWatchlistUnavailable", domain.ErrWatchlistUnavailable, personapi.IsPersonInvalid},
	{"ErrMergeNotProvisional", domain.ErrMergeNotProvisional, personapi.IsPersonInvalid},
	{"ErrMergeIntoInvalid", domain.ErrMergeIntoInvalid, personapi.IsPersonInvalid},
	// ---- missing person or child sub-resource -> 404 PersonNotFound ----
	{"ErrNotFound", domain.ErrNotFound, personapi.IsPersonNotFound},
	{"ErrNameVariantNotFound", domain.ErrNameVariantNotFound, personapi.IsPersonNotFound},
	{"ErrCitizenshipNotFound", domain.ErrCitizenshipNotFound, personapi.IsPersonNotFound},
	{"ErrResidenceNotFound", domain.ErrResidenceNotFound, personapi.IsPersonNotFound},
	{"ErrEmailNotFound", domain.ErrEmailNotFound, personapi.IsPersonNotFound},
	{"ErrPhoneNotFound", domain.ErrPhoneNotFound, personapi.IsPersonNotFound},
	{"ErrCallSignNotFound", domain.ErrCallSignNotFound, personapi.IsPersonNotFound},
	{"ErrMessengerLinkNotFound", domain.ErrMessengerLinkNotFound, personapi.IsPersonNotFound},
	{"ErrSocialAccountNotFound", domain.ErrSocialAccountNotFound, personapi.IsPersonNotFound},
	{"ErrLanguageNotFound", domain.ErrLanguageNotFound, personapi.IsPersonNotFound},
	{"ErrAddressNotFound", domain.ErrAddressNotFound, personapi.IsPersonNotFound},
	{"ErrPartyMembershipNotFound", domain.ErrPartyMembershipNotFound, personapi.IsPersonNotFound},
	{"ErrGovernmentPositionNotFound", domain.ErrGovernmentPositionNotFound, personapi.IsPersonNotFound},
	{"ErrLobbyingNotFound", domain.ErrLobbyingNotFound, personapi.IsPersonNotFound},
	{"ErrExternalReferenceNotFound", domain.ErrExternalReferenceNotFound, personapi.IsPersonNotFound},
	{"ErrRegulatorySanctionNotFound", domain.ErrRegulatorySanctionNotFound, personapi.IsPersonNotFound},
	{"ErrRelationshipNotFound", domain.ErrRelationshipNotFound, personapi.IsPersonNotFound},
	{"ErrEthnicityNotFound", domain.ErrEthnicityNotFound, personapi.IsPersonNotFound},
	{"ErrNameAliasNotFound", domain.ErrNameAliasNotFound, personapi.IsPersonNotFound},
	{"ErrPhysicalDescriptionNotFound", domain.ErrPhysicalDescriptionNotFound, personapi.IsPersonNotFound},
	{"ErrDistinguishingMarkNotFound", domain.ErrDistinguishingMarkNotFound, personapi.IsPersonNotFound},
	{"ErrCryptoWalletNotFound", domain.ErrCryptoWalletNotFound, personapi.IsPersonNotFound},
	{"ErrPersonalityNotFound", domain.ErrPersonalityNotFound, personapi.IsPersonNotFound},
	{"ErrPoliticalLeaningNotFound", domain.ErrPoliticalLeaningNotFound, personapi.IsPersonNotFound},
	{"ErrHealthRecordNotFound", domain.ErrHealthRecordNotFound, personapi.IsPersonNotFound},
	{"ErrInsuranceNotFound", domain.ErrInsuranceNotFound, personapi.IsPersonNotFound},
	{"ErrLegalRecordNotFound", domain.ErrLegalRecordNotFound, personapi.IsPersonNotFound},
	// ---- uniqueness -> 409 PersonConflict ----
	{"ErrCodeConflict", domain.ErrCodeConflict, personapi.IsPersonConflict},
	{"ErrCitizenshipConflict", domain.ErrCitizenshipConflict, personapi.IsPersonConflict},
	{"ErrEmailConflict", domain.ErrEmailConflict, personapi.IsPersonConflict},
	{"ErrPhoneConflict", domain.ErrPhoneConflict, personapi.IsPersonConflict},
	{"ErrCallSignConflict", domain.ErrCallSignConflict, personapi.IsPersonConflict},
	{"ErrMessengerLinkConflict", domain.ErrMessengerLinkConflict, personapi.IsPersonConflict},
	{"ErrSocialAccountConflict", domain.ErrSocialAccountConflict, personapi.IsPersonConflict},
	{"ErrPartnershipConflict", domain.ErrPartnershipConflict, personapi.IsPersonConflict},
	{"ErrRelationshipConflict", domain.ErrRelationshipConflict, personapi.IsPersonConflict},
	{"ErrLanguageConflict", domain.ErrLanguageConflict, personapi.IsPersonConflict},
	// ---- lifecycle -> 409 PersonLifecycleConflict ----
	{"ErrLifecycle", domain.ErrLifecycle, personapi.IsPersonLifecycleConflict},
}

// TestMapErrorClassifiesEverySentinel proves each contracted domain error maps to its typed 4xx Conjure
// error and NOT the `default:` 500 wrap.
func TestMapErrorClassifiesEverySentinel(t *testing.T) {
	ctx := context.Background()
	var svc Service // mapError is a value receiver and reads no Service fields
	for _, tc := range mapErrorContract {
		got := svc.mapError(ctx, tc.err, "test-pid")
		if !tc.want(got) {
			t.Errorf("mapError(%s) = %T (%v); want a typed 4xx Conjure error, not the 500 default", tc.name, got, got)
		}
	}
}

// TestMapErrorContractCoversAllCatalogAndNotFoundSentinels is the drift guard: every catalog/FK
// (`ErrUnknown*`) and child-lookup (`*NotFound`) sentinel defined in the person domain package must
// appear in mapErrorContract (and is therefore verified above to map to a typed 4xx). A new milestone
// that adds such a sentinel fails HERE until it classifies + maps it — instead of silently 500ing.
func TestMapErrorContractCoversAllCatalogAndNotFoundSentinels(t *testing.T) {
	covered := make(map[string]bool, len(mapErrorContract))
	for _, tc := range mapErrorContract {
		covered[tc.name] = true
	}
	re := regexp.MustCompile(`(Err[A-Za-z]+)\s*=\s*errors\.New`)
	files, err := filepath.Glob("../domain/*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no person domain source files found (glob err=%v)", err)
	}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		for _, m := range re.FindAllStringSubmatch(string(src), -1) {
			name := m[1]
			if !strings.HasSuffix(name, "NotFound") && !strings.HasPrefix(name, "ErrUnknown") {
				continue
			}
			if !covered[name] {
				t.Errorf("domain.%s is a catalog/FK/not-found sentinel but is absent from mapErrorContract — "+
					"add a mapError case for it and list it here, else it returns HTTP 500", name)
			}
		}
	}
}
