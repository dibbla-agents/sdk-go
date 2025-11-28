package examples

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	sdk "github.com/dibbla-agents/sdk-go"
	"github.com/dibbla-agents/sdk-go/internal/oauth"
	"github.com/dibbla-agents/sdk-go/internal/state"
	"github.com/dibbla-agents/sdk-go/internal/types"
)

// ============================================================================
// READ GOOGLE SHEETS
// ============================================================================

// ReadGoogleSheetsInputs defines the input for the ReadGoogleSheets function
type ReadGoogleSheetsInputs struct {
	URL string `json:"url"`
}

// ReadGoogleSheetsOutputs defines the output for the ReadGoogleSheets function
type ReadGoogleSheetsOutputs struct {
	SheetContents []SheetContent `json:"sheet_contents"`
	SpreadsheetID string         `json:"spreadsheet_id"`
	Title         string         `json:"title"`
}

// SheetContent represents the contents of a single sheet
type SheetContent struct {
	SheetName string `json:"sheet_name"`
	Cells     string `json:"cells"`     // JSON string: {"A1": "value", "B1": "value", ...}
	RowCount  int    `json:"row_count"` // Number of rows with data
	ColCount  int    `json:"col_count"` // Number of columns with data
}

// Google Sheets API response types
type spreadsheetMetadata struct {
	SpreadsheetID string `json:"spreadsheetId"`
	Properties    struct {
		Title string `json:"title"`
	} `json:"properties"`
	Sheets []struct {
		Properties struct {
			SheetID int    `json:"sheetId"`
			Title   string `json:"title"`
		} `json:"properties"`
	} `json:"sheets"`
}

type sheetValuesResponse struct {
	Range  string     `json:"range"`
	Values [][]string `json:"values"`
}

// ReadGoogleSheetsFunction creates a function that reads all sheets from a Google Sheets URL
func ReadGoogleSheetsFunction() sdk.FunctionBuilder {
	return sdk.NewFunction[ReadGoogleSheetsInputs, ReadGoogleSheetsOutputs](
		"read_google_sheets",
		"1.0.0",
		`Reads all sheets from a Google Sheets spreadsheet.

INPUT:
- url: The full Google Sheets URL (e.g., "https://docs.google.com/spreadsheets/d/1abc.../edit")

OUTPUT:
- title: The spreadsheet title
- spreadsheet_id: The unique spreadsheet identifier
- sheet_contents: Array of sheets, each containing:
  - sheet_name: Name of the sheet tab
  - cells: JSON object mapping cell references to values (e.g., {"A1": "Name", "B1": "Age", "A2": "John", "B2": "30"})
  - row_count: Number of rows with data
  - col_count: Number of columns with data

Note: Empty cells are omitted from the cells object. Use the cell reference (like "B2") to identify specific cells for later updates.`,
	).WithHandler(func(inputs ReadGoogleSheetsInputs, event *types.EventMessage, gs *state.GlobalState) (ReadGoogleSheetsOutputs, error) {
		if gs.OAuth == nil {
			return ReadGoogleSheetsOutputs{}, fmt.Errorf("OAuth client not available")
		}

		// Parse the spreadsheet ID from the URL
		spreadsheetID, err := parseSpreadsheetID(inputs.URL)
		if err != nil {
			return ReadGoogleSheetsOutputs{}, fmt.Errorf("failed to parse spreadsheet URL: %w", err)
		}

		// Get Google OAuth token
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		token, err := gs.OAuth.GetAccessToken(ctx, oauth.ProviderGoogle, event.Run)
		if err != nil {
			return ReadGoogleSheetsOutputs{}, fmt.Errorf("failed to get Google token: %w", err)
		}

		// Create HTTP client with auth token
		client := &http.Client{Timeout: 30 * time.Second}

		// First, get spreadsheet metadata to find all sheet names
		metadata, err := getSpreadsheetMetadata(ctx, client, token.AccessToken, spreadsheetID)
		if err != nil {
			return ReadGoogleSheetsOutputs{}, fmt.Errorf("failed to get spreadsheet metadata: %w", err)
		}

		// Fetch contents of each sheet
		sheetContents := make([]SheetContent, 0, len(metadata.Sheets))
		for _, sheet := range metadata.Sheets {
			cellsJSON, rowCount, colCount, err := getSheetValuesAsJSON(ctx, client, token.AccessToken, spreadsheetID, sheet.Properties.Title)
			if err != nil {
				return ReadGoogleSheetsOutputs{}, fmt.Errorf("failed to get sheet '%s': %w", sheet.Properties.Title, err)
			}

			sheetContents = append(sheetContents, SheetContent{
				SheetName: sheet.Properties.Title,
				Cells:     cellsJSON,
				RowCount:  rowCount,
				ColCount:  colCount,
			})
		}

		return ReadGoogleSheetsOutputs{
			SheetContents: sheetContents,
			SpreadsheetID: spreadsheetID,
			Title:         metadata.Properties.Title,
		}, nil
	}).WithTags("google", "sheets", "spreadsheet", "oauth")
}

// ============================================================================
// UPDATE GOOGLE SHEETS
// ============================================================================

// UpdateGoogleSheetsInputs defines the input for the UpdateGoogleSheets function
type UpdateGoogleSheetsInputs struct {
	URL       string `json:"url"`
	SheetName string `json:"sheet_name"` // Name of the sheet to update
	Range     string `json:"range"`      // A1 notation range, e.g., "A1:C5" or "A1" for single cell
	Values    string `json:"values"`     // JSON 2D array, e.g., [["Name", "Age"], ["John", "30"]]
}

// UpdateGoogleSheetsOutputs defines the output for the UpdateGoogleSheets function
type UpdateGoogleSheetsOutputs struct {
	UpdatedRange  string `json:"updated_range"`
	UpdatedRows   int    `json:"updated_rows"`
	UpdatedCols   int    `json:"updated_columns"`
	UpdatedCells  int    `json:"updated_cells"`
	SpreadsheetID string `json:"spreadsheet_id"`
}

// Google Sheets API update response type
type updateValuesResponse struct {
	SpreadsheetID  string `json:"spreadsheetId"`
	UpdatedRange   string `json:"updatedRange"`
	UpdatedRows    int    `json:"updatedRows"`
	UpdatedColumns int    `json:"updatedColumns"`
	UpdatedCells   int    `json:"updatedCells"`
}

// UpdateGoogleSheetsFunction creates a function that updates cells in a Google Sheet
func UpdateGoogleSheetsFunction() sdk.FunctionBuilder {
	return sdk.NewFunction[UpdateGoogleSheetsInputs, UpdateGoogleSheetsOutputs](
		"update_google_sheets",
		"1.0.0",
		`Updates cells in a Google Sheets spreadsheet.

INPUT:
- url: The full Google Sheets URL (e.g., "https://docs.google.com/spreadsheets/d/1abc.../edit")
- sheet_name: The name of the sheet tab to update (e.g., "Sheet1")
- range: The cell range in A1 notation. Examples:
  - "A1" for a single cell
  - "A1:C3" for a 3x3 range
  - "A:A" for entire column A
- values: A JSON 2D array matching the range dimensions. Examples:
  - Single cell: [["new value"]]
  - Row of 3 cells: [["a", "b", "c"]]
  - 2x2 grid: [["a", "b"], ["c", "d"]]
  - Formula: [["=SUM(A1:A10)"]]

OUTPUT:
- updated_range: The actual range that was updated
- updated_rows: Number of rows updated
- updated_columns: Number of columns updated  
- updated_cells: Total number of cells updated
- spreadsheet_id: The spreadsheet identifier

Note: Values are parsed like user input—formulas execute, dates are recognized, and numbers are parsed automatically.`,
	).WithHandler(func(inputs UpdateGoogleSheetsInputs, event *types.EventMessage, gs *state.GlobalState) (UpdateGoogleSheetsOutputs, error) {
		if gs.OAuth == nil {
			return UpdateGoogleSheetsOutputs{}, fmt.Errorf("OAuth client not available")
		}

		// Parse the spreadsheet ID from the URL
		spreadsheetID, err := parseSpreadsheetID(inputs.URL)
		if err != nil {
			return UpdateGoogleSheetsOutputs{}, fmt.Errorf("failed to parse spreadsheet URL: %w", err)
		}

		// Parse the values JSON string into a 2D array
		var values [][]interface{}
		if err := json.Unmarshal([]byte(inputs.Values), &values); err != nil {
			return UpdateGoogleSheetsOutputs{}, fmt.Errorf("failed to parse values JSON: %w", err)
		}

		// Get Google OAuth token
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		token, err := gs.OAuth.GetAccessToken(ctx, oauth.ProviderGoogle, event.Run)
		if err != nil {
			return UpdateGoogleSheetsOutputs{}, fmt.Errorf("failed to get Google token: %w", err)
		}

		// Create HTTP client with auth token
		client := &http.Client{Timeout: 30 * time.Second}

		// Build the full range with sheet name
		fullRange := fmt.Sprintf("%s!%s", inputs.SheetName, inputs.Range)

		// Call the Sheets API to update values
		result, err := updateSheetValues(ctx, client, token.AccessToken, spreadsheetID, fullRange, values)
		if err != nil {
			return UpdateGoogleSheetsOutputs{}, fmt.Errorf("failed to update sheet: %w", err)
		}

		return UpdateGoogleSheetsOutputs{
			UpdatedRange:  result.UpdatedRange,
			UpdatedRows:   result.UpdatedRows,
			UpdatedCols:   result.UpdatedColumns,
			UpdatedCells:  result.UpdatedCells,
			SpreadsheetID: result.SpreadsheetID,
		}, nil
	}).WithTags("google", "sheets", "spreadsheet", "oauth")
}

// updateSheetValues calls the Google Sheets API to update cell values
func updateSheetValues(ctx context.Context, client *http.Client, accessToken, spreadsheetID, rangeA1 string, values [][]interface{}) (*updateValuesResponse, error) {
	// URL encode the range
	encodedRange := url.PathEscape(rangeA1)
	apiURL := fmt.Sprintf(
		"https://sheets.googleapis.com/v4/spreadsheets/%s/values/%s?valueInputOption=USER_ENTERED",
		spreadsheetID,
		encodedRange,
	)

	// Build request body
	requestBody := map[string]interface{}{
		"range":  rangeA1,
		"values": values,
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result updateValuesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ============================================================================
// SHARED HELPERS
// ============================================================================

// parseSpreadsheetID extracts the spreadsheet ID from a Google Sheets URL
// Example URL: https://docs.google.com/spreadsheets/d/1UbzXDxopFbkRMCPbZjfYOp9ib9kS_fN6iZ8GZQzICrk/edit?gid=0#gid=0
func parseSpreadsheetID(sheetsURL string) (string, error) {
	// First, try to parse as a URL
	parsedURL, err := url.Parse(sheetsURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	// Check if it's a Google Sheets URL
	if !strings.Contains(parsedURL.Host, "docs.google.com") {
		return "", fmt.Errorf("not a Google Sheets URL: host is %s", parsedURL.Host)
	}

	// Extract spreadsheet ID using regex
	// The ID is between /d/ and the next /
	re := regexp.MustCompile(`/spreadsheets/d/([a-zA-Z0-9-_]+)`)
	matches := re.FindStringSubmatch(parsedURL.Path)
	if len(matches) < 2 {
		return "", fmt.Errorf("could not find spreadsheet ID in URL path: %s", parsedURL.Path)
	}

	return matches[1], nil
}

// getSpreadsheetMetadata fetches the spreadsheet metadata including sheet names
func getSpreadsheetMetadata(ctx context.Context, client *http.Client, accessToken, spreadsheetID string) (*spreadsheetMetadata, error) {
	apiURL := fmt.Sprintf("https://sheets.googleapis.com/v4/spreadsheets/%s", spreadsheetID)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var metadata spreadsheetMetadata
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &metadata, nil
}

// colIndexToLetter converts a 0-based column index to A1 notation letter(s)
// Examples: 0 -> "A", 1 -> "B", 25 -> "Z", 26 -> "AA", 27 -> "AB"
func colIndexToLetter(col int) string {
	result := ""
	for col >= 0 {
		result = string(rune('A'+col%26)) + result
		col = col/26 - 1
	}
	return result
}

// getSheetValuesAsJSON fetches the values from a specific sheet and returns them as a JSON string
// with A1 notation keys. Empty cells are omitted.
func getSheetValuesAsJSON(ctx context.Context, client *http.Client, accessToken, spreadsheetID, sheetName string) (string, int, int, error) {
	// URL encode the sheet name to handle special characters
	encodedSheetName := url.PathEscape(sheetName)
	apiURL := fmt.Sprintf("https://sheets.googleapis.com/v4/spreadsheets/%s/values/%s", spreadsheetID, encodedSheetName)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", 0, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := client.Do(req)
	if err != nil {
		return "", 0, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", 0, 0, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var values sheetValuesResponse
	if err := json.NewDecoder(resp.Body).Decode(&values); err != nil {
		return "", 0, 0, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert 2D array to a map with A1 notation keys
	cells := make(map[string]string)
	rowCount := len(values.Values)
	colCount := 0

	for rowIdx, row := range values.Values {
		if len(row) > colCount {
			colCount = len(row)
		}
		for colIdx, value := range row {
			if value != "" { // Skip empty cells
				cellRef := fmt.Sprintf("%s%d", colIndexToLetter(colIdx), rowIdx+1)
				cells[cellRef] = value
			}
		}
	}

	// Convert the map to a JSON string
	cellsJSON, err := json.Marshal(cells)
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to marshal cells to JSON: %w", err)
	}

	return string(cellsJSON), rowCount, colCount, nil
}
