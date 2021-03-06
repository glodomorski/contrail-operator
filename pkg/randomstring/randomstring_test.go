package randomstring

import (
	"regexp"
	"testing"
	"unicode"
)

func hasLettersOnly(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

func isValid(s string) bool {
	re := regexp.MustCompile("^[a-zA-Z0-9_]*$")
	return re.MatchString(s)
}

type TestInterface interface {
	Generate() string
}

func TestRandomstring(t *testing.T) {

	checkGenerateString := func(t *testing.T, testinterface TestInterface) {
		t.Helper()
		got := testinterface.Generate()
		if !hasLettersOnly(got) {
			t.Errorf("failed to verify Generate")
		}
	}

	t.Run("verification", func(t *testing.T) {
		testvalue := RandString{Size: 12}
		checkGenerateString(t, testvalue)
	})

	t.Run("Random string bytes function verification1", func(t *testing.T) {
		value2 := 3
		got := randStringBytes(value2)
		if !isValid(got) {
			t.Errorf("failed to verify random bytes")
		}

	})

}
