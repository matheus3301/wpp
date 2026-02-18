package views

import (
	"github.com/gdamore/tcell/v2"
	wppv1 "github.com/matheus3301/wpp/gen/wpp/v1"
	"github.com/matheus3301/wpp/internal/tui/ui"
	"github.com/rivo/tview"
)

// SearchView provides message search functionality.
type SearchView struct {
	*tview.Flex
	theme   *ui.Theme
	input   *tview.InputField
	results *tview.Table
	onQuery func(query string)
	data    []*wppv1.SearchResult
}

// NewSearchView creates a new search view.
func NewSearchView(theme *ui.Theme) *SearchView {
	input := tview.NewInputField().
		SetLabel(" Search: ").
		SetFieldWidth(0)
	input.SetBorderColor(theme.BorderColor)
	input.SetBackgroundColor(theme.BgColor)
	input.SetFieldBackgroundColor(theme.BgColor)
	input.SetFieldTextColor(theme.FgColor)
	input.SetLabelColor(theme.MenuKeyColor)

	results := tview.NewTable().
		SetSelectable(true, false).
		SetBorders(false).
		SetFixed(1, 0)
	results.SetBorder(true)
	results.SetBorderColor(theme.BorderColor)
	results.SetBackgroundColor(theme.BgColor)
	results.SetTitle(" Results ")
	results.SetTitleColor(theme.TitleColor)
	results.SetSelectedStyle(tcell.StyleDefault.
		Foreground(theme.TableCursorFg).
		Background(theme.TableCursorBg))

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(input, 1, 0, true).
		AddItem(results, 0, 1, false)

	sv := &SearchView{
		Flex:    flex,
		theme:   theme,
		input:   input,
		results: results,
	}

	return sv
}

// Name implements Component.
func (sv *SearchView) Name() string { return "Search" }

// Init implements Component.
func (sv *SearchView) Init() {}

// Start implements Component.
func (sv *SearchView) Start() {}

// Stop implements Component.
func (sv *SearchView) Stop() {}

// Hints implements Component.
func (sv *SearchView) Hints() []ui.MenuHint {
	return []ui.MenuHint{
		{Key: "Enter", Description: "Search/Open"},
		{Key: "Esc", Description: "Back"},
		{Key: ":", Description: "Command"},
	}
}

// SetOnQuery sets the callback when a search query is submitted.
func (sv *SearchView) SetOnQuery(fn func(query string)) {
	sv.onQuery = fn
	sv.input.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter && sv.onQuery != nil {
			sv.onQuery(sv.input.GetText())
		}
	})
}

// Update refreshes search results.
func (sv *SearchView) Update(results []*wppv1.SearchResult) {
	sv.data = results
	sv.results.Clear()

	// Header.
	headers := []string{" CHAT", " SNIPPET", " TIME"}
	for col, h := range headers {
		sv.results.SetCell(0, col, tview.NewTableCell(h).
			SetSelectable(false).
			SetTextColor(sv.theme.TableHeaderFg).
			SetBackgroundColor(sv.theme.TableHeaderBg).
			SetAttributes(tcell.AttrBold))
	}

	for i, r := range results {
		row := i + 1
		chatJID := ""
		ts := ""
		if r.Message != nil {
			chatJID = r.Message.ChatJid
			ts = formatTimestamp(r.Message.TimestampUnixMs)
		}
		sv.results.SetCell(row, 0, tview.NewTableCell(" "+tview.Escape(chatJID)).SetMaxWidth(25).SetTextColor(sv.theme.FgColor))
		sv.results.SetCell(row, 1, tview.NewTableCell(" "+tview.Escape(sanitizeForTerminal(r.Snippet))).SetExpansion(1).SetTextColor(sv.theme.FgColor))
		sv.results.SetCell(row, 2, tview.NewTableCell(" "+ts).SetMaxWidth(12).SetTextColor(sv.theme.FgColor))
	}
}

// SelectedResult returns the chat JID and message ID of the selected result.
func (sv *SearchView) SelectedResult() (string, string) {
	row, _ := sv.results.GetSelection()
	idx := row - 1
	if idx >= 0 && idx < len(sv.data) {
		r := sv.data[idx]
		if r.Message != nil {
			return r.Message.ChatJid, r.Message.Id
		}
	}
	return "", ""
}

// Input returns the search input field.
func (sv *SearchView) Input() *tview.InputField {
	return sv.input
}

// Results returns the results table.
func (sv *SearchView) Results() *tview.Table {
	return sv.results
}
