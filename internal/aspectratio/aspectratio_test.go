package aspectratio

import "testing"

func TestNamedAliasesMatchNumericRatios(t *testing.T) {
	aliases := map[string]string{
		"widescreen": "16:9",
		"cinematic":  "2.39:1",
		"landscape":  "4:3",
		"portrait":   "3:4",
	}
	for alias, numeric := range aliases {
		t.Run(alias, func(t *testing.T) {
			aliasWidth, aliasHeight, err := Dimensions(alias)
			if err != nil {
				t.Fatalf("Dimensions(%q) error = %v", alias, err)
			}
			numericWidth, numericHeight, err := Dimensions(numeric)
			if err != nil {
				t.Fatalf("Dimensions(%q) error = %v", numeric, err)
			}
			if aliasWidth != numericWidth || aliasHeight != numericHeight {
				t.Errorf("Dimensions(%q) = %g×%g, want Dimensions(%q) = %g×%g", alias, aliasWidth, aliasHeight, numeric, numericWidth, numericHeight)
			}
		})
	}
}
