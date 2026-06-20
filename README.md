# BMD Tools

A terminal user interface (TUI) client for searching and pivot-analyzing Birth, Marriage, and Death (BMD) records from regional UKBMD local databases. Built in Go using the Bubble Tea framework.

Currently supports querying four major sites in northern/north-west England in parallel:
- **Lancashire BMD** (`lancashirebmd.org.uk`)
- **Cheshire BMD** (`cheshirebmd.org.uk`)
- **Cumbria BMD** (`cumbriabmd.org.uk`)
- **Yorkshire BMD** (`yorkshirebmd.org.uk`)

## Features

### 1. Interactive TUI Search
- Search form supporting input name parsing (e.g., `"First Last"` or `"Last, First"`).
- Persisted year bounds and single-key reset (`Ctrl+R`) to default ranges.

### 2. Birth & Marriage Searches
- Filter by Name, Mother's Maiden Name, Spouse Name, and Year/Range.
- **Ignore Blank MMN by Default**: In Birth search, specifying an MMN automatically excludes records with a blank index field.
- **Widen MMN (`w`)**: Press `w` on birth results to re-run the search *including* blank MMNs.

### 3. Multi-County Search Scope (`a`)
- Searches default to the local **Lancashire BMD** index.
- Press **`a`** on any birth or marriage results page to immediately widen the query and run it across all four databases (**Lancashire**, **Cheshire**, **Cumbria**, and **Yorkshire**) in parallel. Results are automatically merged, sorted, and presented in a single unified view.

### 4. Automatic Life Event Checking
- Pressing `Enter` on any birth record automatically searches marriages (16-80 years after birth) and deaths (up to 110 years after birth).
- Displays matched counts dynamically in the detail panel. If exactly 1 match is found, details of the marriage or death record are displayed inline.
- **Middle Name Fallback**: If a lookup fails to find matches, it automatically retries without middle names.

### 5. Advanced Search Pivots
- **Marriage Pivot (`m`)**: Press `m` on any birth record to pivot to the Marriage Search form.
  - Automatically calculates and populates the spouse's expected name and marriage year window. If a cached check returned exactly one marriage record, pressing `m` skips the form and jumps to the marriage results screen.
- **Children Search Pivot (`c`)**: Press `c` on any marriage record to search for children of that marriage.
  - Uses a gender heuristic to infer child surname and mother's maiden name, setting a search range of 10 years before the wedding to 40 years after.

---

## Usage

### Prerequisites
- Go 1.18 or higher installed on your machine.

### Installation
Clone the repository, build, and run:
```bash
# Build the binary
go build -o bmd-tools main.go

# Run the CLI
./bmd-tools
```

Alternatively, run directly:
```bash
go run main.go
```

### Hotkeys and Controls

| Location | Key | Action |
| --- | --- | --- |
| **Main Menu** | `b` | Open Birth Search form |
| | `m` | Open Marriage Search form |
| | `q` / `Ctrl+C` | Quit |
| **Search Forms** | `Tab` / `Arrows` | Move between input fields |
| | `Enter` | Submit search query |
| | `Ctrl+R` | Reset year bounds to defaults |
| | `Esc` | Return to previous state / main menu |
| **Search Results** | `Up` / `Down` | Scroll through matching records |
| | `Esc` | Go back to edit search form |
| | `Enter` (on Birth) | Fetch matches for marriages and deaths |
| | `m` (on Birth) | Pivot to marriage search |
| | `w` (on Birth) | Widen search to include blank Mother's Maiden Name records |
| | `a` (on Birth/Marriage)| Widen search scope across all supported databases (Lancashire, Cheshire, Cumbria, Yorkshire) |
| | `c` (on Marriage) | Pivot to searching for children of the marriage |

---

## Limitations

- **Source Specific**: Designed for querying Lancashire, Cheshire, Cumbria, and Yorkshire BMD databases. It will not retrieve records from other counties or the national General Register Office (GRO) databases.
- **Date Boundaries**:
  - Birth searches are restricted to 1837-2007.
  - Marriage searches are restricted to 1837-2022.
  - Death searches are restricted to 1837-2009.
- **Name Matches**: All queries map to the website's `exact` match setting, meaning spelling variations will not be captured automatically.
- **Gender Heuristics**: The `c` (Children) pivot infers the mother's maiden name by comparing the forename of the marriage partners against a list of common historic female names. If both or neither match, it default-falls back to standard field mappings.
