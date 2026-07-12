package transport

import (
	"reflect"
	"testing"
)

// TestLocaleProjection covers the D-i18n locale= projection (review R-19): absent/empty/blank input
// means "no projection" (return all locales), and a real list projects each label map to just those
// locales, intersected with what is stored.
func TestLocaleProjection(t *testing.T) {
	full := map[string]string{"ukr": "Синій", "eng": "Blue", "spa": "Azul", "por": "Azul"}

	t.Run("no projection returns the full map unchanged", func(t *testing.T) {
		for name, in := range map[string][]string{
			"nil":      nil,
			"empty":    {},
			"allblank": {"", ""},
		} {
			want := localeProjection(in)
			if want != nil {
				t.Fatalf("%s: localeProjection must yield nil (no projection), got %v", name, want)
			}
			got := projectLocales(full, want)
			if !reflect.DeepEqual(got, full) {
				t.Fatalf("%s: projectLocales(nil) must return the full map, got %v", name, got)
			}
		}
	})

	t.Run("projects to the requested subset", func(t *testing.T) {
		got := projectLocales(full, localeProjection([]string{"ukr", "eng"}))
		want := map[string]string{"ukr": "Синій", "eng": "Blue"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("projection = %v, want %v", got, want)
		}
	})

	t.Run("requested-but-absent locales are simply omitted (intersection)", func(t *testing.T) {
		got := projectLocales(full, localeProjection([]string{"ukr", "deu"}))
		want := map[string]string{"ukr": "Синій"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("projection = %v, want %v (deu not stored → omitted)", got, want)
		}
	})
}
