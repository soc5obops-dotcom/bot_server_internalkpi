package sheets

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	sheetsapi "google.golang.org/api/sheets/v4"
)

type Client struct {
	svc      *sheetsapi.Service
	sheetID  string
	tokenSrc oauth2.TokenSource
}

func New(ctx context.Context, credentialsPath, credentialsJSON, sheetID string) (*Client, error) {
	credentialsBytes := []byte(credentialsJSON)
	if len(credentialsBytes) == 0 {
		fileBytes, err := os.ReadFile(credentialsPath)
		if err != nil {
			return nil, fmt.Errorf("read google credentials: %w", err)
		}
		credentialsBytes = fileBytes
	}
	creds, err := google.CredentialsFromJSON(ctx, credentialsBytes, sheetsapi.SpreadsheetsScope, "https://www.googleapis.com/auth/drive.readonly")
	if err != nil {
		return nil, fmt.Errorf("parse google credentials: %w", err)
	}
	svc, err := sheetsapi.NewService(ctx, option.WithCredentialsJSON(credentialsBytes), option.WithScopes(sheetsapi.SpreadsheetsScope, "https://www.googleapis.com/auth/drive.readonly"))
	if err != nil {
		return nil, err
	}
	return &Client{svc: svc, sheetID: sheetID, tokenSrc: creds.TokenSource}, nil
}

func (c *Client) Values(ctx context.Context, tab, rng string) ([][]string, error) {
	resp, err := c.svc.Spreadsheets.Values.Get(c.sheetID, quoteTab(tab)+"!"+rng).Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	out := make([][]string, len(resp.Values))
	for i, row := range resp.Values {
		out[i] = make([]string, len(row))
		for j, cell := range row {
			out[i][j] = fmt.Sprint(cell)
		}
	}
	return out, nil
}

func (c *Client) GroupIDs(ctx context.Context, tab string) ([]string, error) {
	values, err := c.Values(ctx, tab, "A2:A")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var ids []string
	for _, row := range values {
		if len(row) == 0 {
			continue
		}
		id := strings.TrimSpace(row[0])
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (c *Client) UpsertGroupID(ctx context.Context, tab, groupID string) error {
	ids, err := c.GroupIDs(ctx, tab)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if id == groupID {
			return nil
		}
	}
	ids = append(ids, groupID)
	return c.writeGroupIDs(ctx, tab, ids)
}

func (c *Client) RemoveGroupID(ctx context.Context, tab, groupID string) error {
	ids, err := c.GroupIDs(ctx, tab)
	if err != nil {
		return err
	}
	var filtered []string
	for _, id := range ids {
		if id != groupID {
			filtered = append(filtered, id)
		}
	}
	return c.writeGroupIDs(ctx, tab, filtered)
}

func (c *Client) NormalizeGroupIDs(ctx context.Context, tab string) error {
	ids, err := c.GroupIDs(ctx, tab)
	if err != nil {
		return err
	}
	return c.writeGroupIDs(ctx, tab, ids)
}

func (c *Client) writeGroupIDs(ctx context.Context, tab string, ids []string) error {
	sort.Strings(ids)
	clearRange := quoteTab(tab) + "!A2:A"
	_, err := c.svc.Spreadsheets.Values.Clear(c.sheetID, clearRange, &sheetsapi.ClearValuesRequest{}).Context(ctx).Do()
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	values := make([][]any, len(ids))
	for i, id := range ids {
		values[i] = []any{id}
	}
	_, err = c.svc.Spreadsheets.Values.Update(c.sheetID, quoteTab(tab)+"!A2", &sheetsapi.ValueRange{
		Values: values,
	}).ValueInputOption("RAW").Context(ctx).Do()
	return err
}

func (c *Client) SheetGID(ctx context.Context, tab string) (int64, error) {
	resp, err := c.svc.Spreadsheets.Get(c.sheetID).Fields("sheets(properties(sheetId,title))").Context(ctx).Do()
	if err != nil {
		return 0, err
	}
	for _, sheet := range resp.Sheets {
		if sheet.Properties != nil && sheet.Properties.Title == tab {
			return sheet.Properties.SheetId, nil
		}
	}
	return 0, fmt.Errorf("tab %q not found", tab)
}

func (c *Client) Token(ctx context.Context) (string, error) {
	tok, err := c.tokenSrc.Token()
	if err != nil {
		return "", err
	}
	return tok.AccessToken, nil
}

func quoteTab(tab string) string {
	return "'" + strings.ReplaceAll(tab, "'", "''") + "'"
}
