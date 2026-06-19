package tui

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mhutchinson/bmd-tools/pkg/bmd"
)

type State int

const (
	StateMenu State = iota
	StateForm
	StateLoading
	StateResults
	StateError
)

type SearchType int

const (
	TypeBirths SearchType = iota
	TypeMarriages
)

// Msg types
type searchBirthsMsg struct {
	results []bmd.BirthRecord
	err     error
}

type searchMarriagesMsg struct {
	results []bmd.MarriageRecord
	err     error
}

type lifeEventsMsg struct {
	birthRef    string
	marriages   []bmd.MarriageRecord
	marriageErr error
	deaths      []bmd.DeathRecord
	deathErr    error
}

type LifeEventsCacheEntry struct {
	loading     bool
	marriages   []bmd.MarriageRecord
	deaths      []bmd.DeathRecord
	marriageErr error
	deathErr    error
}

// Model represents the TUI application state.
type Model struct {
	state                   State
	searchType              SearchType
	inputs                  []textinput.Model
	focusIndex              int
	err                     error
	birthResults            []bmd.BirthRecord
	marriageResults         []bmd.MarriageRecord
	cursor                  int // Selected row in results
	scroll                  int // Scroll offset in results
	spinner                 spinner.Model
	width                   int
	height                  int
	searchName              string
	searchMaiden            string
	searchYears             string
	persistedYearsBirths    string
	persistedYearsMarriages string
	lifeEventsCache         map[string]LifeEventsCacheEntry
	ignoreBlankMMN          bool
	searchSites             []string
}

// Styles
var (
	mauve     = lipgloss.Color("#cba6f7")
	lavender  = lipgloss.Color("#b4befe")
	blue      = lipgloss.Color("#89b4fa")
	pink      = lipgloss.Color("#f5c2e7")
	green     = lipgloss.Color("#a6e3a1")
	red       = lipgloss.Color("#f38ba8")
	gray      = lipgloss.Color("#585b70")
	darkGray  = lipgloss.Color("#313244")
	lightGray = lipgloss.Color("#cdd6f4")

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffffff")).
			Background(mauve).
			Padding(0, 2).
			MarginBottom(1)

	headerStyle = lipgloss.NewStyle().
			Foreground(blue).
			Bold(true)

	focusedStyle = lipgloss.NewStyle().
			Foreground(lavender)

	blurredStyle = lipgloss.NewStyle().
			Foreground(gray)

	errorStyle = lipgloss.NewStyle().
			Foreground(red).
			Bold(true)

	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(gray).
			Padding(1, 2)

	selectedRowStyle = lipgloss.NewStyle().
				Background(darkGray).
				Foreground(pink).
				Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(gray).
			Italic(true)
)

// NewModel initializes the TUI model.
func NewModel() Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(mauve)

	inputs := make([]textinput.Model, 3)
	inputs[0] = textinput.New()
	inputs[0].Placeholder = "John Smith"
	inputs[0].Focus()
	inputs[0].PromptStyle = focusedStyle
	inputs[0].TextStyle = focusedStyle

	inputs[1] = textinput.New()
	inputs[1].Placeholder = "Parker"
	inputs[1].PromptStyle = blurredStyle
	inputs[1].TextStyle = blurredStyle

	inputs[2] = textinput.New()
	inputs[2].Placeholder = "1837-2007"
	inputs[2].SetValue("1837-2007")
	inputs[2].PromptStyle = blurredStyle
	inputs[2].TextStyle = blurredStyle

	return Model{
		state:                   StateMenu,
		searchType:              TypeBirths,
		inputs:                  inputs,
		spinner:                 s,
		persistedYearsBirths:    "1837-2007",
		persistedYearsMarriages: "1837-2022",
		lifeEventsCache:         make(map[string]LifeEventsCacheEntry),
		searchSites:             []string{"lancashire"},
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) performSearch() tea.Cmd {
	nameVal := m.inputs[0].Value()
	field1Val := m.inputs[1].Value()
	yearsVal := m.inputs[2].Value()

	forename, surname := bmd.ParseName(nameVal)

	if m.searchType == TypeBirths {
		startYear, endYear, _ := bmd.ParseYearRange(yearsVal, 2007)
		return func() tea.Msg {
			var allResults []bmd.BirthRecord
			var errs []string

			type chanResult struct {
				results []bmd.BirthRecord
				err     error
				site    string
			}
			ch := make(chan chanResult, len(m.searchSites))
			for _, site := range m.searchSites {
				go func(s string) {
					params := bmd.SearchParams{
						Surname:        surname,
						Forename:       forename,
						MaidenSurname:  field1Val,
						StartYear:      startYear,
						EndYear:        endYear,
						IgnoreBlankMMN: m.ignoreBlankMMN,
						Site:           s,
					}
					res, err := bmd.SearchBirths(context.Background(), params)
					ch <- chanResult{results: res, err: err, site: s}
				}(site)
			}

			for i := 0; i < len(m.searchSites); i++ {
				r := <-ch
				if r.err != nil {
					errs = append(errs, fmt.Sprintf("%s: %v", r.site, r.err))
				} else {
					allResults = append(allResults, r.results...)
				}
			}

			var finalErr error
			if len(errs) > 0 {
				finalErr = fmt.Errorf("errors querying sites: %s", strings.Join(errs, "; "))
			}

			SortBirthRecords(allResults)

			return searchBirthsMsg{results: allResults, err: finalErr}
		}
	} else {
		spouseForename, spouseSurname := bmd.ParseName(field1Val)
		startYear, endYear, _ := bmd.ParseYearRange(yearsVal, 2022)
		return func() tea.Msg {
			var allResults []bmd.MarriageRecord
			var errs []string

			type chanResult struct {
				results []bmd.MarriageRecord
				err     error
				site    string
			}
			ch := make(chan chanResult, len(m.searchSites))
			for _, site := range m.searchSites {
				go func(s string) {
					params := bmd.MarriageSearchParams{
						Surname:        surname,
						Forename:       forename,
						SpouseSurname:  spouseSurname,
						SpouseForename: spouseForename,
						StartYear:      startYear,
						EndYear:        endYear,
						Site:           s,
					}
					res, err := bmd.SearchMarriages(context.Background(), params)
					ch <- chanResult{results: res, err: err, site: s}
				}(site)
			}

			for i := 0; i < len(m.searchSites); i++ {
				r := <-ch
				if r.err != nil {
					errs = append(errs, fmt.Sprintf("%s: %v", r.site, r.err))
				} else {
					allResults = append(allResults, r.results...)
				}
			}

			var finalErr error
			if len(errs) > 0 {
				finalErr = fmt.Errorf("errors querying sites: %s", strings.Join(errs, "; "))
			}

			SortMarriageRecords(allResults)

			return searchMarriagesMsg{results: allResults, err: finalErr}
		}
	}
}

func searchLifeEventsCmd(r bmd.BirthRecord) tea.Cmd {
	return func() tea.Msg {
		by, _ := strconv.Atoi(r.Year)
		if by == 0 {
			by = 1900 // Fallback
		}

		// Marriages search: 16-80 years after birth
		mStart := by + 16
		mEnd := by + 80
		if mStart < 1837 {
			mStart = 1837
		}
		if mEnd > 2022 {
			mEnd = 2022
		}

		mParams := bmd.MarriageSearchParams{
			Surname:   r.Surname,
			Forename:  r.Forename,
			StartYear: mStart,
			EndYear:   mEnd,
		}

		// Deaths search: all available years up to 110 after birth
		dStart := by
		dEnd := by + 110
		if dStart < 1837 {
			dStart = 1837
		}
		if dEnd > 2009 {
			dEnd = 2009
		}

		var yob string
		if by != 0 && r.Year != "" {
			yob = r.Year
		}

		dParams := bmd.DeathSearchParams{
			Surname:     r.Surname,
			Forename:    r.Forename,
			StartYear:   dStart,
			EndYear:     dEnd,
			YearOfBirth: yob,
		}

		ctx := context.Background()

		// Run marriages search
		marriages, mErr := bmd.SearchMarriages(ctx, mParams)
		words := strings.Fields(r.Forename)
		hasMiddleNames := len(words) > 1
		var firstForename string
		if hasMiddleNames {
			firstForename = words[0]
		}

		if mErr == nil && len(marriages) == 0 && hasMiddleNames {
			fallbackParams := mParams
			fallbackParams.Forename = firstForename
			marriages, mErr = bmd.SearchMarriages(ctx, fallbackParams)
		}

		// Run deaths search
		deaths, dErr := bmd.SearchDeaths(ctx, dParams)
		if dErr == nil && len(deaths) == 0 && hasMiddleNames {
			fallbackParams := dParams
			fallbackParams.Forename = firstForename
			deaths, dErr = bmd.SearchDeaths(ctx, fallbackParams)
		}

		return lifeEventsMsg{
			birthRef:    r.Reference,
			marriages:   marriages,
			marriageErr: mErr,
			deaths:      deaths,
			deathErr:    dErr,
		}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}

		switch m.state {
		case StateMenu:
			switch msg.String() {
			case "b":
				m.searchType = TypeBirths
				m.state = StateForm
				m.err = nil
				m.focusIndex = 0
				m.inputs[0].SetValue("")
				m.inputs[0].Placeholder = "John Smith"
				m.inputs[1].SetValue("")
				m.inputs[1].Placeholder = "Parker"
				m.inputs[2].SetValue(m.persistedYearsBirths)
				m.inputs[2].Placeholder = "1837-2007"
				for i := range m.inputs {
					if i == 0 {
						m.inputs[i].Focus()
						m.inputs[i].PromptStyle = focusedStyle
						m.inputs[i].TextStyle = focusedStyle
					} else {
						m.inputs[i].Blur()
						m.inputs[i].PromptStyle = blurredStyle
						m.inputs[i].TextStyle = blurredStyle
					}
				}
				return m, nil
			case "m":
				m.searchType = TypeMarriages
				m.state = StateForm
				m.err = nil
				m.focusIndex = 0
				m.inputs[0].SetValue("")
				m.inputs[0].Placeholder = "John Smith"
				m.inputs[1].SetValue("")
				m.inputs[1].Placeholder = "Barlow or Hilda Barlow"
				m.inputs[2].SetValue(m.persistedYearsMarriages)
				m.inputs[2].Placeholder = "1837-2022"
				for i := range m.inputs {
					if i == 0 {
						m.inputs[i].Focus()
						m.inputs[i].PromptStyle = focusedStyle
						m.inputs[i].TextStyle = focusedStyle
					} else {
						m.inputs[i].Blur()
						m.inputs[i].PromptStyle = blurredStyle
						m.inputs[i].TextStyle = blurredStyle
					}
				}
				return m, nil
			case "q":
				return m, tea.Quit
			}

		case StateForm:
			switch msg.String() {
			case "esc":
				m.state = StateMenu
				return m, nil

			case "ctrl+r":
				if m.searchType == TypeBirths {
					m.inputs[2].SetValue("1837-2007")
					m.persistedYearsBirths = "1837-2007"
				} else {
					m.inputs[2].SetValue("1837-2022")
					m.persistedYearsMarriages = "1837-2022"
				}
				return m, nil

			case "tab", "down":
				m.inputs[m.focusIndex].Blur()
				m.inputs[m.focusIndex].PromptStyle = blurredStyle
				m.inputs[m.focusIndex].TextStyle = blurredStyle

				m.focusIndex = (m.focusIndex + 1) % len(m.inputs)

				m.inputs[m.focusIndex].Focus()
				m.inputs[m.focusIndex].PromptStyle = focusedStyle
				m.inputs[m.focusIndex].TextStyle = focusedStyle
				return m, nil

			case "shift+tab", "up":
				m.inputs[m.focusIndex].Blur()
				m.inputs[m.focusIndex].PromptStyle = blurredStyle
				m.inputs[m.focusIndex].TextStyle = blurredStyle

				m.focusIndex--
				if m.focusIndex < 0 {
					m.focusIndex = len(m.inputs) - 1
				}

				m.inputs[m.focusIndex].Focus()
				m.inputs[m.focusIndex].PromptStyle = focusedStyle
				m.inputs[m.focusIndex].TextStyle = focusedStyle
				return m, nil

			case "enter":
				// Validate
				nameVal := m.inputs[0].Value()
				field1Val := m.inputs[1].Value()
				yearsVal := m.inputs[2].Value()

				if m.searchType == TypeBirths {
					if strings.TrimSpace(nameVal) == "" && strings.TrimSpace(field1Val) == "" {
						m.err = fmt.Errorf("Name or Mother's Maiden Name is required")
						return m, nil
					}
					_, surname := bmd.ParseName(nameVal)
					if surname == "" && strings.TrimSpace(field1Val) == "" {
						m.err = fmt.Errorf("Surname or Mother's Maiden Name is required")
						return m, nil
					}
					_, _, err := bmd.ParseYearRange(yearsVal, 2007)
					if err != nil {
						m.err = err
						return m, nil
					}
					m.persistedYearsBirths = yearsVal
					m.ignoreBlankMMN = (strings.TrimSpace(field1Val) != "")
				} else {
					if strings.TrimSpace(nameVal) == "" {
						m.err = fmt.Errorf("Name is required")
						return m, nil
					}
					_, surname := bmd.ParseName(nameVal)
					if surname == "" {
						m.err = fmt.Errorf("Surname is required")
						return m, nil
					}
					_, _, err := bmd.ParseYearRange(yearsVal, 2022)
					if err != nil {
						m.err = err
						return m, nil
					}
					m.persistedYearsMarriages = yearsVal
				}

				m.searchName = nameVal
				m.searchMaiden = field1Val
				m.searchYears = yearsVal
				m.searchSites = []string{"lancashire"}

				m.state = StateLoading
				m.err = nil
				return m, tea.Batch(m.spinner.Tick, m.performSearch())
			}

			// Pass keypress to current focused input
			m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
			return m, cmd

		case StateLoading:
			// Spinner ticks or searchMsg response handles it

		case StateResults:
			pageSize := m.height - 13
			if pageSize < 1 {
				pageSize = 5
			}

			resultsLen := 0
			if m.searchType == TypeBirths {
				resultsLen = len(m.birthResults)
			} else {
				resultsLen = len(m.marriageResults)
			}

			switch msg.String() {
			case "esc":
				m.state = StateForm
				return m, nil
			case "q":
				return m, tea.Quit
			case "enter":
				if m.searchType == TypeBirths && len(m.birthResults) > 0 {
					r := m.birthResults[m.cursor]
					entry, exists := m.lifeEventsCache[r.Reference]
					if !exists || entry.marriageErr != nil || entry.deathErr != nil {
						m.lifeEventsCache[r.Reference] = LifeEventsCacheEntry{loading: true}
						return m, searchLifeEventsCmd(r)
					}
				}
				return m, nil
			case "w":
				if m.searchType == TypeBirths && strings.TrimSpace(m.searchMaiden) != "" {
					if m.ignoreBlankMMN {
						m.ignoreBlankMMN = false
						m.state = StateLoading
						m.err = nil
						return m, tea.Batch(m.spinner.Tick, m.performSearch())
					}
				}
				return m, nil
			case "a":
				if len(m.searchSites) < 3 {
					m.searchSites = []string{"lancashire", "cheshire", "cumbria"}
					m.state = StateLoading
					m.err = nil
					return m, tea.Batch(m.spinner.Tick, m.performSearch())
				}
				return m, nil
			case "up":
				if m.cursor > 0 {
					m.cursor--
					if m.cursor < m.scroll {
						m.scroll = m.cursor
					}
				}
				return m, nil
			case "down":
				if m.cursor < resultsLen-1 {
					m.cursor++
					if m.cursor >= m.scroll+pageSize {
						m.scroll = m.cursor - pageSize + 1
					}
				}
				return m, nil
			case "m":
				// Pivot from birth results to marriage search
				if m.searchType == TypeBirths && len(m.birthResults) > 0 {
					r := m.birthResults[m.cursor]

					// Check if we have a cached lookup with exactly one marriage record
					entry, exists := m.lifeEventsCache[r.Reference]
					if exists && !entry.loading && entry.marriageErr == nil && len(entry.marriages) == 1 {
						// Jump directly to the marriage results screen
						m.searchType = TypeMarriages
						m.state = StateResults
						m.marriageResults = entry.marriages
						m.cursor = 0
						m.scroll = 0

						// Store search params for the query banner display
						m.searchName = fmt.Sprintf("%s %s", r.Forename, r.Surname)
						m.searchMaiden = "" // No spouse name filter applied
						by, _ := strconv.Atoi(r.Year)
						if by == 0 {
							by = 1900
						}
						mStart := by + 16
						mEnd := by + 80
						if mStart < 1837 {
							mStart = 1837
						}
						if mEnd > 2022 {
							mEnd = 2022
						}
						m.searchYears = fmt.Sprintf("%d-%d", mStart, mEnd)
						return m, nil
					}

					// Fallback to the marriage search form
					by, _ := strconv.Atoi(r.Year)
					if by == 0 {
						by = 1900 // Fallback
					}
					startYear := by + 16
					endYear := by + 80
					if startYear < 1837 {
						startYear = 1837
					}
					if endYear > 2022 {
						endYear = 2022
					}

					m.searchType = TypeMarriages
					m.state = StateForm
					m.err = nil
					m.focusIndex = 1 // Focus spouse name input

					m.inputs[0].SetValue(fmt.Sprintf("%s %s", r.Forename, r.Surname))
					m.inputs[0].Placeholder = "John Smith"
					m.inputs[1].SetValue("")
					m.inputs[1].Placeholder = "Barlow or Hilda Barlow"
					m.inputs[2].SetValue(fmt.Sprintf("%d-%d", startYear, endYear))
					m.inputs[2].Placeholder = "1837-2022"

					for i := range m.inputs {
						if i == 1 {
							m.inputs[i].Focus()
							m.inputs[i].PromptStyle = focusedStyle
							m.inputs[i].TextStyle = focusedStyle
						} else {
							m.inputs[i].Blur()
							m.inputs[i].PromptStyle = blurredStyle
							m.inputs[i].TextStyle = blurredStyle
						}
					}
					return m, nil
				}
			case "c":
				// Pivot from marriage results to children (birth) search
				if m.searchType == TypeMarriages && len(m.marriageResults) > 0 {
					r := m.marriageResults[m.cursor]
					wy, _ := strconv.Atoi(r.Year)
					if wy == 0 {
						wy = 1920 // Fallback
					}
					startYear := wy - 10
					endYear := wy + 40
					if startYear < 1837 {
						startYear = 1837
					}
					if endYear > 2007 {
						endYear = 2007
					}

					m.searchType = TypeBirths
					m.state = StateForm
					m.err = nil
					m.focusIndex = 0 // Focus name input

					childSurname := r.Surname
					motherMaiden := r.SpouseSurname
					if IsCommonFemaleName(r.Forename) {
						childSurname = r.SpouseSurname
						motherMaiden = r.Surname
					}

					m.inputs[0].SetValue(childSurname)
					m.inputs[0].Placeholder = "John Smith"
					m.inputs[1].SetValue(motherMaiden)
					m.inputs[1].Placeholder = "Parker"
					m.inputs[2].SetValue(fmt.Sprintf("%d-%d", startYear, endYear))
					m.inputs[2].Placeholder = "1837-2007"

					for i := range m.inputs {
						if i == 0 {
							m.inputs[i].Focus()
							m.inputs[i].PromptStyle = focusedStyle
							m.inputs[i].TextStyle = focusedStyle
						} else {
							m.inputs[i].Blur()
							m.inputs[i].PromptStyle = blurredStyle
							m.inputs[i].TextStyle = blurredStyle
						}
					}
					return m, nil
				}
			}

		case StateError:
			switch msg.String() {
			case "esc":
				m.state = StateForm
				return m, nil
			case "q":
				return m, tea.Quit
			}
		}

	case spinner.TickMsg:
		if m.state == StateLoading {
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case searchBirthsMsg:
		if m.state == StateLoading {
			if msg.err != nil {
				m.state = StateError
				m.err = msg.err
			} else {
				m.state = StateResults
				m.birthResults = msg.results
				m.cursor = 0
				m.scroll = 0
			}
			return m, nil
		}

	case searchMarriagesMsg:
		if m.state == StateLoading {
			if msg.err != nil {
				m.state = StateError
				m.err = msg.err
			} else {
				m.state = StateResults
				m.marriageResults = msg.results
				m.cursor = 0
				m.scroll = 0
			}
			return m, nil
		}

	case lifeEventsMsg:
		m.lifeEventsCache[msg.birthRef] = LifeEventsCacheEntry{
			loading:     false,
			marriages:   msg.marriages,
			deaths:      msg.deaths,
			marriageErr: msg.marriageErr,
			deathErr:    msg.deathErr,
		}
		return m, nil
	}

	return m, nil
}

func IsCommonFemaleName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return false
	}
	first := parts[0]

	femaleNames := map[string]bool{
		"mary": true, "elizabeth": true, "sarah": true, "margaret": true,
		"jane": true, "ann": true, "anne": true, "annie": true,
		"alice": true, "florence": true, "emily": true, "eliza": true,
		"ellen": true, "helen": true, "martha": true, "dorothy": true,
		"ada": true, "clara": true, "hannah": true, "edith": true,
		"louisa": true, "rose": true, "grace": true, "lily": true,
		"ruth": true, "maud": true, "harriet": true, "beatrice": true,
		"ethel": true, "elsie": true, "hilda": true, "agnes": true,
		"doris": true, "lilian": true, "gertrude": true, "gladys": true,
		"marjorie": true, "nellie": true, "bertha": true,
	}
	return femaleNames[first]
}

func pad(s string, width int) string {
	runes := []rune(s)
	if len(runes) > width {
		return string(runes[:width-3]) + "..."
	}
	return s + strings.Repeat(" ", width-len(runes))
}

func (m Model) View() string {
	var s strings.Builder

	// Title Banner
	if m.searchType == TypeBirths {
		s.WriteString(titleStyle.Render("BMD Tools - Lancashire Birth Search"))
	} else {
		s.WriteString(titleStyle.Render("BMD Tools - Lancashire Marriage Search"))
	}
	s.WriteString("\n\n")

	switch m.state {
	case StateMenu:
		menuText := "Welcome to BMD Tools CLI.\n\n" +
			"Press " + lipgloss.NewStyle().Foreground(pink).Bold(true).Render("b") + " to search Births (Lancashire BMD)\n" +
			"Press " + lipgloss.NewStyle().Foreground(pink).Bold(true).Render("m") + " to search Marriages (Lancashire BMD)\n" +
			"Press " + lipgloss.NewStyle().Foreground(pink).Bold(true).Render("q") + " to quit"

		s.WriteString(borderStyle.Render(menuText))

	case StateForm:
		var form strings.Builder
		if m.searchType == TypeBirths {
			form.WriteString(headerStyle.Render("Search Births Form") + "\n\n")
		} else {
			form.WriteString(headerStyle.Render("Search Marriages Form") + "\n\n")
		}

		// Render inputs
		var labels []string
		if m.searchType == TypeBirths {
			labels = []string{
				"Full Name (Forename Surname or Surname, Forename):",
				"Mother's Maiden Name (Optional):                  ",
				"Year or Range (1837-2007, e.g. 1900 or 1900-1910):",
			}
		} else {
			labels = []string{
				"Full Name (Forename Surname or Surname, Forename):",
				"Spouse's Name (Optional, e.g. John Smith or Smith):",
				"Year or Range (1837-2022, e.g. 1920 or 1920-1930):",
			}
		}

		for i := range m.inputs {
			form.WriteString(fmt.Sprintf("%s\n%s\n\n", labels[i], m.inputs[i].View()))
		}

		if m.err != nil {
			form.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)) + "\n\n")
		}

		form.WriteString(helpStyle.Render("[tab/arrows] Navigate  [enter] Search  [ctrl+r] Reset Year  [esc] Main Menu"))

		s.WriteString(borderStyle.Render(form.String()))

	case StateLoading:
		var loading string
		if m.searchType == TypeBirths {
			loading = fmt.Sprintf("%s Fetching birth records from lancashirebmd.org.uk...\n\n", m.spinner.View())
		} else {
			loading = fmt.Sprintf("%s Fetching marriage records from lancashirebmd.org.uk...\n\n", m.spinner.View())
		}
		s.WriteString(borderStyle.Render(loading))

	case StateResults:
		pageSize := m.height - 13
		if pageSize < 1 {
			pageSize = 5
		}

		resultsLen := 0
		if m.searchType == TypeBirths {
			resultsLen = len(m.birthResults)
		} else {
			resultsLen = len(m.marriageResults)
		}

		s.WriteString(headerStyle.Render(fmt.Sprintf("Search Results (%d matches)", resultsLen)) + "\n")
		queryLabel := "Mother's Maiden Name"
		if m.searchType == TypeMarriages {
			queryLabel = "Spouse Name"
		}
		sitesStr := "Lancashire"
		if len(m.searchSites) == 3 {
			sitesStr = "All NW"
		} else if len(m.searchSites) == 1 {
			switch m.searchSites[0] {
			case "cheshire":
				sitesStr = "Cheshire"
			case "cumbria":
				sitesStr = "Cumbria"
			}
		}
		queryDetails := fmt.Sprintf("Query: Name=%q, %s=%q, Years=%q, Site=%s", m.searchName, queryLabel, m.searchMaiden, m.searchYears, sitesStr)
		if m.searchType == TypeBirths && strings.TrimSpace(m.searchMaiden) != "" {
			if m.ignoreBlankMMN {
				queryDetails += " (Ignoring blank MMN)"
			} else {
				queryDetails += " (Including blank MMN)"
			}
		}
		s.WriteString(helpStyle.Render(queryDetails) + "\n\n")

		if resultsLen == 0 {
			var options []string
			options = append(options, "[esc] to search again")
			if m.searchType == TypeBirths && strings.TrimSpace(m.searchMaiden) != "" && m.ignoreBlankMMN {
				options = append(options, "[w] to widen search (include blank MMN)")
			}
			if len(m.searchSites) < 3 {
				options = append(options, "[a] to widen to All NW")
			}

			var msg string
			if len(options) == 1 {
				msg = fmt.Sprintf("No records matching the search criteria were found.\n\nPress %s.", options[0])
			} else if len(options) == 2 {
				msg = fmt.Sprintf("No records matching the search criteria were found.\n\nPress %s, or %s.", options[0], options[1])
			} else {
				msg = fmt.Sprintf("No records matching the search criteria were found.\n\nPress %s, %s, or %s.", options[0], options[1], options[2])
			}
			s.WriteString(borderStyle.Render(msg))
			return s.String()
		}

		// Render Table
		endIndex := m.scroll + pageSize
		if endIndex > resultsLen {
			endIndex = resultsLen
		}

		if m.searchType == TypeBirths {
			// Widths: Surname: 12, Forename: 18, Maiden: 14, Year: 6, Sub-District: 18, Reference: 14
			headerRow := fmt.Sprintf("%s %s %s %s %s %s",
				pad("Surname", 12),
				pad("Forename(s)", 18),
				pad("Mother's Maiden", 14),
				pad("Year", 6),
				pad("Sub-District", 18),
				pad("Reference", 14),
			)
			s.WriteString(lipgloss.NewStyle().Underline(true).Bold(true).Foreground(blue).Render(headerRow) + "\n")

			for i := m.scroll; i < endIndex; i++ {
				r := m.birthResults[i]
				rowText := fmt.Sprintf("%s %s %s %s %s %s",
					pad(r.Surname, 12),
					pad(r.Forename, 18),
					pad(r.MotherMaidenName, 14),
					pad(r.Year, 6),
					pad(r.SubDistrict, 18),
					pad(r.Reference, 14),
				)

				if i == m.cursor {
					s.WriteString(selectedRowStyle.Render("> "+rowText) + "\n")
				} else {
					s.WriteString("  " + rowText + "\n")
				}
			}
		} else {
			// Marriages Table
			// Surname: 12, Forename: 14, Spouse Surname: 14, Spouse Forename: 14, Year: 6, Church/Venue: 20, Reference: 12
			headerRow := fmt.Sprintf("%s %s %s %s %s %s %s",
				pad("Surname", 12),
				pad("Forename(s)", 14),
				pad("Spouse Surname", 14),
				pad("Spouse Forename", 14),
				pad("Year", 6),
				pad("Venue/Church", 20),
				pad("Reference", 12),
			)
			s.WriteString(lipgloss.NewStyle().Underline(true).Bold(true).Foreground(blue).Render(headerRow) + "\n")

			for i := m.scroll; i < endIndex; i++ {
				r := m.marriageResults[i]
				rowText := fmt.Sprintf("%s %s %s %s %s %s %s",
					pad(r.Surname, 12),
					pad(r.Forename, 14),
					pad(r.SpouseSurname, 14),
					pad(r.SpouseForename, 14),
					pad(r.Year, 6),
					pad(r.ChurchRegisterOffice, 20),
					pad(r.Reference, 12),
				)

				if i == m.cursor {
					s.WriteString(selectedRowStyle.Render("> "+rowText) + "\n")
				} else {
					s.WriteString("  " + rowText + "\n")
				}
			}
		}

		// Scroll indicator
		s.WriteString(fmt.Sprintf("\nShowing %d-%d of %d matches\n\n", m.scroll+1, endIndex, resultsLen))

		// Detail Panel for selected record
		if m.searchType == TypeBirths {
			if m.cursor < len(m.birthResults) {
				r := m.birthResults[m.cursor]
				var detail strings.Builder
				detail.WriteString(headerStyle.Render("Record Details") + "\n")
				detail.WriteString(fmt.Sprintf("Name:        %s, %s\n", r.Surname, r.Forename))
				mmn := r.MotherMaidenName
				if mmn == "" {
					mmn = "(Blank/Not Indexed)"
				}
				detail.WriteString(fmt.Sprintf("Mother MMN:  %s\n", mmn))
				detail.WriteString(fmt.Sprintf("Year:        %s\n", r.Year))
				detail.WriteString(fmt.Sprintf("Sub-District: %s\n", r.SubDistrict))
				detail.WriteString(fmt.Sprintf("Registers At: %s\n", r.RegistersAt))
				detail.WriteString(fmt.Sprintf("Reference:   %s\n", r.Reference))

				// Display marriages/deaths search status or results
				detail.WriteString("\n")
				entry, exists := m.lifeEventsCache[r.Reference]
				if !exists {
					detail.WriteString(helpStyle.Render("Press [Enter] to check marriages & deaths for this person.\n"))
				} else if entry.loading {
					detail.WriteString(focusedStyle.Render("⌛ Checking marriages & deaths...\n"))
				} else {
					// Render Marriage status
					if entry.marriageErr != nil {
						detail.WriteString(errorStyle.Render(fmt.Sprintf("Marriages: Error checking marriages (%v)\n", entry.marriageErr)))
					} else {
						mMatches := len(entry.marriages)
						if mMatches == 0 {
							detail.WriteString("Marriages:   0 matches\n")
						} else if mMatches == 1 {
							mRecord := entry.marriages[0]
							detail.WriteString(fmt.Sprintf("Marriages:   1 match (m. %s to %s %s, Venue: %s, Ref: %s)\n",
								mRecord.Year, mRecord.SpouseForename, mRecord.SpouseSurname, mRecord.ChurchRegisterOffice, mRecord.Reference))
						} else {
							detail.WriteString(fmt.Sprintf("Marriages:   %d matches\n", mMatches))
						}
					}

					// Render Death status
					if entry.deathErr != nil {
						detail.WriteString(errorStyle.Render(fmt.Sprintf("Deaths:      Error checking deaths (%v)\n", entry.deathErr)))
					} else {
						dMatches := len(entry.deaths)
						if dMatches == 0 {
							detail.WriteString("Deaths:      0 matches\n")
						} else if dMatches == 1 {
							dRecord := entry.deaths[0]
							ageStr := dRecord.Age
							if ageStr == "" {
								ageStr = "Age unknown"
							} else {
								ageStr = "Age " + ageStr
							}
							detail.WriteString(fmt.Sprintf("Deaths:      1 match (d. %s, %s, Sub-District: %s, Ref: %s)\n",
								dRecord.Year, ageStr, dRecord.SubDistrict, dRecord.Reference))
						} else {
							detail.WriteString(fmt.Sprintf("Deaths:      %d matches\n", dMatches))
						}
					}
				}

				s.WriteString(borderStyle.BorderForeground(lavender).Render(detail.String()) + "\n")
			}
			helpText := "[↑/↓] Navigate  [enter] Find Marriages/Deaths  [m] Pivot to Marriage Search  [esc] Search again  [q] Quit"
			if strings.TrimSpace(m.searchMaiden) != "" && m.ignoreBlankMMN {
				helpText += "  [w] Widen MMN"
			}
			if len(m.searchSites) < 3 {
				helpText += "  [a] Widen to All NW"
			}
			s.WriteString(helpStyle.Render(helpText))
		} else {
			if m.cursor < len(m.marriageResults) {
				r := m.marriageResults[m.cursor]
				var detail strings.Builder
				detail.WriteString(headerStyle.Render("Record Details") + "\n")
				detail.WriteString(fmt.Sprintf("Name:        %s, %s\n", r.Surname, r.Forename))
				detail.WriteString(fmt.Sprintf("Spouse Name: %s, %s\n", r.SpouseSurname, r.SpouseForename))
				detail.WriteString(fmt.Sprintf("Year:        %s\n", r.Year))
				detail.WriteString(fmt.Sprintf("Venue/Church: %s\n", r.ChurchRegisterOffice))
				detail.WriteString(fmt.Sprintf("Registers At: %s\n", r.RegistersAt))
				detail.WriteString(fmt.Sprintf("Reference:   %s\n", r.Reference))

				s.WriteString(borderStyle.BorderForeground(lavender).Render(detail.String()) + "\n")
			}
			helpText := "[↑/↓] Navigate  [c] Find Children (Birth Search)  [esc] Search again  [q] Quit"
			if len(m.searchSites) < 3 {
				helpText += "  [a] Widen to All NW"
			}
			s.WriteString(helpStyle.Render(helpText))
		}

	case StateError:
		var errView strings.Builder
		errView.WriteString(errorStyle.Render("Search Failed") + "\n\n")
		errView.WriteString(fmt.Sprintf("%v\n\n", m.err))
		errView.WriteString(helpStyle.Render("[esc] Go back  [q] Quit"))
		s.WriteString(borderStyle.Render(errView.String()))
	}

	return s.String()
}

func SortBirthRecords(records []bmd.BirthRecord) {
	sort.Slice(records, func(i, j int) bool {
		if records[i].Year != records[j].Year {
			return records[i].Year < records[j].Year
		}
		if records[i].Surname != records[j].Surname {
			return records[i].Surname < records[j].Surname
		}
		return records[i].Forename < records[j].Forename
	})
}

func SortMarriageRecords(records []bmd.MarriageRecord) {
	sort.Slice(records, func(i, j int) bool {
		if records[i].Year != records[j].Year {
			return records[i].Year < records[j].Year
		}
		if records[i].Surname != records[j].Surname {
			return records[i].Surname < records[j].Surname
		}
		return records[i].Forename < records[j].Forename
	})
}
