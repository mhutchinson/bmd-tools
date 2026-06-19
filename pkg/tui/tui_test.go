package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestIgnoreBlankMMN(t *testing.T) {
	// 1. Test when mother's maiden name is provided, m.ignoreBlankMMN should be true.
	m := NewModel()
	m.searchType = TypeBirths
	m.state = StateForm
	m.inputs[0].SetValue("John Smith")
	m.inputs[1].SetValue("Parker") // Maiden Name
	m.inputs[2].SetValue("1900-1910")

	// Trigger enter key press
	resModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updatedModel := resModel.(Model)

	if !updatedModel.ignoreBlankMMN {
		t.Errorf("Expected ignoreBlankMMN to be true when Mother's Maiden Name is provided")
	}

	// 2. Test when mother's maiden name is blank, m.ignoreBlankMMN should be false.
	m2 := NewModel()
	m2.searchType = TypeBirths
	m2.state = StateForm
	m2.inputs[0].SetValue("John Smith")
	m2.inputs[1].SetValue("") // Blank Maiden Name
	m2.inputs[2].SetValue("1900-1910")

	resModel2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updatedModel2 := resModel2.(Model)

	if updatedModel2.ignoreBlankMMN {
		t.Errorf("Expected ignoreBlankMMN to be false when Mother's Maiden Name is empty")
	}

	// 3. Test that pressing 'w' on results page toggles ignoreBlankMMN to false and triggers search
	m3 := NewModel()
	m3.searchType = TypeBirths
	m3.state = StateResults
	m3.searchName = "John Smith"
	m3.searchMaiden = "Parker"
	m3.searchYears = "1900-1910"
	m3.ignoreBlankMMN = true

	resModel3, cmd := m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	updatedModel3 := resModel3.(Model)

	if updatedModel3.ignoreBlankMMN {
		t.Errorf("Expected ignoreBlankMMN to become false when 'w' key is pressed")
	}
	if cmd == nil {
		t.Errorf("Expected a non-nil search command when pressing 'w'")
	}
	if updatedModel3.state != StateLoading {
		t.Errorf("Expected state to transition to StateLoading, got %v", updatedModel3.state)
	}
}
