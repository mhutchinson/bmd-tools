package bmd

import (
	"testing"
)

func TestParseName(t *testing.T) {
	tests := []struct {
		input    string
		forename string
		surname  string
	}{
		{"", "", ""},
		{"Smith", "", "Smith"},
		{"John Smith", "John", "Smith"},
		{"Mary Anne Smith", "Mary Anne", "Smith"},
		{"Smith, John", "John", "Smith"},
		{"Smith, Mary Anne", "Mary Anne", "Smith"},
		{"  John   Smith  ", "John", "Smith"},
	}

	for _, tt := range tests {
		forename, surname := ParseName(tt.input)
		if forename != tt.forename || surname != tt.surname {
			t.Errorf("ParseName(%q) = (%q, %q); want (%q, %q)", tt.input, forename, surname, tt.forename, tt.surname)
		}
	}
}

func TestParseYearRange(t *testing.T) {
	tests := []struct {
		input     string
		startYear int
		endYear   int
		wantErr   bool
	}{
		{"", 1837, 2007, false}, // default range
		{"1900", 1900, 1900, false},
		{"1900-1910", 1900, 1910, false},
		{"1910-1900", 1900, 1910, false}, // swaps if start > end
		{"1830", 0, 0, true},             // out of bounds (min is 1837)
		{"2007", 2007, 2007, false},
		{"2008", 0, 0, true},      // out of bounds (max is 2007)
		{"1900-2008", 0, 0, true}, // out of bounds end
		{"abc", 0, 0, true},       // invalid integer
		{"1900-abc", 0, 0, true},  // invalid range integer
	}

	for _, tt := range tests {
		s, e, err := ParseYearRange(tt.input, 2007)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseYearRange(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr {
			if s != tt.startYear || e != tt.endYear {
				t.Errorf("ParseYearRange(%q) = (%d, %d); want (%d, %d)", tt.input, s, e, tt.startYear, tt.endYear)
			}
		}
	}
}
