package views

import (
	"github.com/gdamore/tcell/v2"
	wppv1 "github.com/matheus3301/wpp/gen/wpp/v1"
	"github.com/rivo/tview"
)

// SearchView provides message search functionality.
type SearchView struct {
	*tview.Flex
	input   *tview.InputField
	results *tview.Table
	onQuery func(query string)
	data    []*wppv1.SearchResult
}

// NewSearchView creates a new search view.
func NewSearchView() *SearchView {
	input := tview.NewInputField().
		SetLabel(" Search: ").
		SetFieldWidth(0)

	results := tview.NewTable().
		SetSelectable(true, false).
		SetBorders(false)
	results.SetBorder(true).SetTitle(" Results ")

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(input, 1, 0, true).
		AddItem(results, 0, 1, false)

	sv := &SearchView{
		Flex:    flex,
		input:   input,
		results: results,
	}

	return sv
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

	sv.results.SetCell(0, 0, tview.NewTableCell(" Chat").SetSelectable(false).SetTextColor(tview.Styles.SecondaryTextColor))
	sv.results.SetCell(0, 1, tview.NewTableCell(" Snippet").SetSelectable(false).SetTextColor(tview.Styles.SecondaryTextColor))

	for i, r := range results {
		row := i + 1
		chatJID := ""
		if r.Message != nil {
			chatJID = r.Message.ChatJid
		}
		sv.results.SetCell(row, 0, tview.NewTableCell(" "+chatJID).SetMaxWidth(25))
		sv.results.SetCell(row, 1, tview.NewTableCell(" "+r.Snippet).SetExpansion(1))
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
