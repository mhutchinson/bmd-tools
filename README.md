# BMD Tools

A terminal user interface (TUI) client for searching and pivot-analyzing Birth, Marriage, and Death (BMD) records from the Lancashire BMD website ([lancashirebmd.org.uk](https://www.lancashirebmd.org.uk/)). Built in Go using the Bubble Tea framework.

## Features

### 1. Interactive TUI Search
- Search form supporting input name parsing (e.g., `"First Last"` or `"Last, First"`).
- Persisted year bounds and single-key reset (`Ctrl+R`) to default ranges.

### 2. Birth Search (1837-2007)
- Filter by Name, Mother's Maiden Name (MMN), and Year/Range.
- **Ignore Blank MMN by Default**: If a Mother's Maiden Name is specified in the search form, the tool automatically restricts search results to records with an indexed MMN (reducing noise from blank indices).
- **Widen Search (`w`)**: Press `w` on the results page to quickly re-run the search *including* records with blank MMNs without returning to the form.

### 3. Automatic Life Event Checking
- Pressing `Enter` on any birth record automatically searches marriages (16-80 years after birth) and deaths (up to 110 years after birth).
- Displays matched counts dynamically at the bottom detail panel. If exactly 1 match is found, the details of the marriage or death record are displayed inline.
- **Middle Name Fallback**: If a lookup fails to find any matches, it automatically retries without middle names, which are commonly abbreviated or omitted in marriage and death indices.

### 4. Marriage Search (1837-2022)
- Search by Spouse Name, Surname, Forename, and Year/Range.
- **Marriage Pivot (`m`)**: Press `m` on any birth record to pivot to the Marriage Search form.
  - Automatically calculates and populates the spouse's expected name and marriage year window (16 to 80 years post-birth).
  - If a cached background check yielded exactly one marriage record, pressing `m` skips the form entirely and immediately jumps to the single record.

### 5. Children Search Pivot (`c`)
- From any marriage record, press `c` to search for children of that marriage.
- The tool uses a gender heuristic to infer child surname and mother's maiden name, populating the Birth Search form.
- The year search range is automatically set to 10 years before the wedding to 40 years after.

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
| | `w` (on Birth) | Widen search (include blank Mother's Maiden Name records) |
| | `c` (on Marriage) | Pivot to searching for children of the marriage |

---

## Limitations

- **Source Specific**: Designed exclusively for querying Lancashire BMD. It will not retrieve records from other counties or General Register Office (GRO) databases.
- **Date Boundaries**:
  - Birth searches are restricted to 1837-2007.
  - Marriage searches are restricted to 1837-2022.
  - Death searches are restricted to 1837-2009.
- **Name Matches**: All queries map to the website's `exact` match setting, meaning variations or spelling errors will not be captured automatically.
- **Gender Heuristics**: The `c` (Children) pivot infers the mother's maiden name by comparing the forename of the marriage partners against a list of common historic female names. If both or neither match, it default-falls back to standard field mappings.
