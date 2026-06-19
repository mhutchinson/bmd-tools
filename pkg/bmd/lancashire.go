package bmd

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// BirthRecord represents a birth record fetched from Lancashire BMD.
type BirthRecord struct {
	Type             string `json:"type"`
	Surname          string `json:"surname"`
	Forename         string `json:"forename"`
	MotherMaidenName string `json:"mother_maiden_name"`
	Year             string `json:"year"`
	SubDistrict      string `json:"sub_district"`
	Region           string `json:"region"`
	RegistersAt      string `json:"registers_at"`
	Reference        string `json:"reference"`
}

// SearchParams contains parameters for searching the BMD index.
type SearchParams struct {
	Surname        string
	Forename       string
	MaidenSurname  string
	StartYear      int
	EndYear        int
	IgnoreBlankMMN bool
}

// ParseName splits a full name input into forename and surname.
// Format support:
// - "Surname, Forename" (comma separated)
// - "Forename Surname" (space separated, where the last word is the surname)
func ParseName(input string) (forename, surname string) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", ""
	}

	// Comma separated (Last, First)
	if strings.Contains(input, ",") {
		parts := strings.SplitN(input, ",", 2)
		surname = strings.TrimSpace(parts[0])
		forename = strings.TrimSpace(parts[1])
		return forename, surname
	}

	// Space separated (First Last)
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return "", parts[0]
	}

	// Last word is surname, preceding words are forename
	surname = parts[len(parts)-1]
	forename = strings.Join(parts[:len(parts)-1], " ")
	return forename, surname
}

// ParseYearRange parses a year or year range (e.g. "1900" or "1900-1910") up to a maxYear.
func ParseYearRange(input string, maxYear int) (startYear, endYear int, err error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return 1837, maxYear, nil
	}

	if strings.Contains(input, "-") {
		parts := strings.SplitN(input, "-", 2)
		sStr := strings.TrimSpace(parts[0])
		eStr := strings.TrimSpace(parts[1])

		s, err := strconv.Atoi(sStr)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid start year: %s", sStr)
		}
		e, err := strconv.Atoi(eStr)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid end year: %s", eStr)
		}

		if s < 1837 || s > maxYear || e < 1837 || e > maxYear {
			return 0, 0, fmt.Errorf("years must be between 1837 and %d", maxYear)
		}
		if s > e {
			return e, s, nil
		}
		return s, e, nil
	}

	y, err := strconv.Atoi(input)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid year: %s", input)
	}
	if y < 1837 || y > maxYear {
		return 0, 0, fmt.Errorf("year must be between 1837 and %d", maxYear)
	}
	return y, y, nil
}

// SearchBirths performs a search on the Lancashire BMD birth indexes.
func SearchBirths(ctx context.Context, params SearchParams) ([]BirthRecord, error) {
	if params.Surname == "" && params.MaidenSurname == "" {
		return nil, fmt.Errorf("either surname or mother's maiden name is required")
	}

	if params.StartYear < 1837 || params.EndYear > 2007 || params.StartYear > params.EndYear {
		return nil, fmt.Errorf("invalid year range: %d-%d", params.StartYear, params.EndYear)
	}

	// Prepare POST data
	data := url.Values{}
	data.Set("county", "lancashire")
	data.Set("lang", "")
	for y := params.StartYear; y <= params.EndYear; y++ {
		data.Add("year_date[]", strconv.Itoa(y))
	}
	data.Set("year_plus_minus_val", "0")
	data.Set("search_region[]", "All")
	data.Set("sort_by", "alpha")
	data.Set("search_district", "all")
	data.Set("surname", params.Surname)
	data.Set("initial", params.Forename)
	data.Set("maiden_surname", params.MaidenSurname)
	if params.IgnoreBlankMMN {
		data.Set("ignore_blank_mmn", "yes")
	} else {
		data.Set("ignore_blank_mmn", "no")
	}
	data.Set("ignore_flag", "1")
	data.Set("match", "exact")
	data.Set("csv_or_list", "file")
	data.Set("submit", "Display Results")

	// Construct request
	req, err := http.NewRequestWithContext(ctx, "POST", "https://www.lancashirebmd.org.uk/birthsearch.php", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	// Send request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status: %d %s", resp.StatusCode, resp.Status)
	}

	// Read and filter lines to only include CSV rows starting with "Birth" or the header "Type,"
	scanner := bufio.NewScanner(resp.Body)
	var csvBuilder strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Type,") || strings.HasPrefix(line, "\"Birth\"") {
			csvBuilder.WriteString(line + "\n")
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse CSV
	csvReader := csv.NewReader(strings.NewReader(csvBuilder.String()))
	csvReader.FieldsPerRecord = -1
	csvReader.LazyQuotes = true
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	if len(records) <= 1 {
		// Only header or empty
		return nil, nil
	}

	var results []BirthRecord
	// Skip header row
	for _, row := range records[1:] {
		if len(row) < 10 {
			continue // Malformed row
		}
		results = append(results, BirthRecord{
			Type:             row[0],
			Surname:          row[1],
			Forename:         row[2],
			MotherMaidenName: row[3],
			Year:             row[5],
			SubDistrict:      row[6],
			Region:           row[7],
			RegistersAt:      row[8],
			Reference:        row[9],
		})
	}

	return results, nil
}

// MarriageRecord represents a marriage record fetched from Lancashire BMD.
type MarriageRecord struct {
	Type                 string `json:"type"`
	Surname              string `json:"surname"`
	Forename             string `json:"forename"`
	SpouseSurname        string `json:"spouse_surname"`
	SpouseForename       string `json:"spouse_forename"`
	Year                 string `json:"year"`
	ChurchRegisterOffice string `json:"church_register_office"`
	Region               string `json:"region"`
	RegistersAt          string `json:"registers_at"`
	Reference            string `json:"reference"`
}

// MarriageSearchParams contains parameters for searching the marriages index.
type MarriageSearchParams struct {
	Surname        string
	Forename       string
	SpouseSurname  string
	SpouseForename string
	StartYear      int
	EndYear        int
}

// SearchMarriages performs a search on the Lancashire BMD marriage indexes.
func SearchMarriages(ctx context.Context, params MarriageSearchParams) ([]MarriageRecord, error) {
	if params.Surname == "" {
		return nil, fmt.Errorf("surname is required")
	}

	if params.StartYear < 1837 || params.EndYear > 2022 || params.StartYear > params.EndYear {
		return nil, fmt.Errorf("invalid year range: %d-%d", params.StartYear, params.EndYear)
	}

	// Prepare POST data
	data := url.Values{}
	data.Set("county", "lancashire")
	data.Set("lang", "")
	for y := params.StartYear; y <= params.EndYear; y++ {
		data.Add("year_date[]", strconv.Itoa(y))
	}
	data.Set("year_plus_minus_val", "0")
	data.Set("search_region[]", "All")
	data.Set("sort_by", "alpha")
	data.Set("search_district", "all")
	data.Set("surname", params.Surname)
	data.Set("initial", params.Forename)
	data.Set("spouse_surname", params.SpouseSurname)
	data.Set("spouse_initial", params.SpouseForename)
	data.Set("match", "exact")
	data.Set("csv_or_list", "file")
	data.Set("submit", "Display Results")

	// Construct request
	req, err := http.NewRequestWithContext(ctx, "POST", "https://www.lancashirebmd.org.uk/marriagesearch.php", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	// Send request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status: %d %s", resp.StatusCode, resp.Status)
	}

	// Read and filter lines to only include CSV rows starting with "Marriage" or the header "Type,"
	scanner := bufio.NewScanner(resp.Body)
	var csvBuilder strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Type,") || strings.HasPrefix(line, "\"Marriage\"") {
			csvBuilder.WriteString(line + "\n")
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse CSV
	csvReader := csv.NewReader(strings.NewReader(csvBuilder.String()))
	csvReader.FieldsPerRecord = -1
	csvReader.LazyQuotes = true
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	if len(records) <= 1 {
		// Only header or empty
		return nil, nil
	}

	var results []MarriageRecord
	// Skip header row
	for _, row := range records[1:] {
		if len(row) < 10 {
			continue // Malformed row
		}
		results = append(results, MarriageRecord{
			Type:                 row[0],
			Surname:              row[1],
			Forename:             row[2],
			SpouseSurname:        row[3],
			SpouseForename:       row[4],
			Year:                 row[5],
			ChurchRegisterOffice: row[6],
			Region:               row[7],
			RegistersAt:          row[8],
			Reference:            row[9],
		})
	}

	return results, nil
}

// DeathRecord represents a death record fetched from Lancashire BMD.
type DeathRecord struct {
	Type        string `json:"type"`
	Surname     string `json:"surname"`
	Forename    string `json:"forename"`
	Age         string `json:"age"`
	Year        string `json:"year"`
	SubDistrict string `json:"sub_district"`
	Region      string `json:"region"`
	RegistersAt string `json:"registers_at"`
	Reference   string `json:"reference"`
}

// DeathSearchParams contains parameters for searching the deaths index.
type DeathSearchParams struct {
	Surname     string
	Forename    string
	StartYear   int
	EndYear     int
	YearOfBirth string
}

// SearchDeaths performs a search on the Lancashire BMD death indexes.
func SearchDeaths(ctx context.Context, params DeathSearchParams) ([]DeathRecord, error) {
	if params.Surname == "" {
		return nil, fmt.Errorf("surname is required")
	}

	if params.StartYear < 1837 || params.EndYear > 2009 || params.StartYear > params.EndYear {
		return nil, fmt.Errorf("invalid year range: %d-%d", params.StartYear, params.EndYear)
	}

	// Prepare POST data
	data := url.Values{}
	data.Set("county", "lancashire")
	data.Set("lang", "")
	for y := params.StartYear; y <= params.EndYear; y++ {
		data.Add("year_date[]", strconv.Itoa(y))
	}
	data.Set("year_plus_minus_val", "0")
	data.Set("search_region[]", "All")
	data.Set("sort_by", "alpha")
	data.Set("search_district", "all")
	data.Set("surname", params.Surname)
	data.Set("initial", params.Forename)
	if params.YearOfBirth != "" {
		data.Set("age_at_death", params.YearOfBirth)
		data.Set("plus_minus_val", "1")
	}
	data.Set("match", "exact")
	data.Set("csv_or_list", "file")
	data.Set("submit", "Display Results")

	// Construct request
	req, err := http.NewRequestWithContext(ctx, "POST", "https://www.lancashirebmd.org.uk/deathsearch.php", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	// Send request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status: %d %s", resp.StatusCode, resp.Status)
	}

	// Read and filter lines to only include CSV rows starting with "Death" or the header "Type,"
	scanner := bufio.NewScanner(resp.Body)
	var csvBuilder strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Type,") || strings.HasPrefix(line, "\"Death\"") {
			csvBuilder.WriteString(line + "\n")
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse CSV
	csvReader := csv.NewReader(strings.NewReader(csvBuilder.String()))
	csvReader.FieldsPerRecord = -1
	csvReader.LazyQuotes = true
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	if len(records) <= 1 {
		// Only header or empty
		return nil, nil
	}

	var results []DeathRecord
	// Skip header row
	for _, row := range records[1:] {
		if len(row) < 10 {
			continue // Malformed row
		}
		results = append(results, DeathRecord{
			Type:        row[0],
			Surname:     row[1],
			Forename:    row[2],
			Age:         row[3],
			Year:        row[5],
			SubDistrict: row[6],
			Region:      row[7],
			RegistersAt: row[8],
			Reference:   row[9],
		})
	}

	return results, nil
}
