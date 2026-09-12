// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2026 Tulir Asokan, buihongduc132
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package tui

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"go.mau.fi/mauview"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/gomuks/pkg/rpc/store"
)

// UserTagModal is a multi-select user tagging modal popup wrapping mauview.InputArea and mauview.TextView.
// It supports fuzzy search filtering, multi-selection tracking via unique User ID, and dual confirmation.
type UserTagModal struct {
	mauview.Component

	lock      sync.RWMutex
	inputLock sync.Mutex

	parent    *MainView
	container *mauview.Box
	search    *mauview.InputArea
	results   *mauview.TextView

	members       []*store.AutocompleteMemberEntry
	matches       []*store.AutocompleteMemberEntry
	selected      int
	selectedUsers map[id.UserID]struct{}
	selectedOrder []id.UserID

	onConfirm    func(selectedUsers []string)
	dismissed    bool
	renderedText string
}

// NewUserTagModal creates and initializes a UserTagModal centered on screen.
func NewUserTagModal(parent *MainView, members []*store.AutocompleteMemberEntry, onConfirm func(selectedUsers []string)) *UserTagModal {
	m := &UserTagModal{
		parent:        parent,
		search:        mauview.NewInputArea(),
		results:       mauview.NewTextView().SetRegions(true),
		members:       members,
		selected:      0,
		selectedUsers: make(map[id.UserID]struct{}),
		selectedOrder: make([]id.UserID, 0),
		onConfirm:     onConfirm,
	}

	m.search.SetPlaceholder("Search users...")
	m.search.SetTextColor(tcell.ColorWhite).SetBackgroundColor(tcell.ColorDarkCyan)
	m.search.SetChangedFunc(func(text string) {
		m.filter(text)
	})
	m.search.Focus()

	flex := mauview.NewFlex().
		SetDirection(mauview.FlexRow).
		AddFixedComponent(m.search, 1).
		AddProportionalComponent(m.results, 1)

	m.container = mauview.NewBox(flex).
		SetBorder(true).
		SetTitle(" Tag Users ").
		SetBlurCaptureFunc(func() bool {
			m.cancel()
			return true
		})

	m.Component = mauview.Center(m.container, 50, 15).SetAlwaysFocusChild(true)
	m.filter("")
	return m
}

// NewUserTagModalFromRoom creates a UserTagModal populated with members from the given room store.
func NewUserTagModalFromRoom(parent *MainView, room *store.RoomStore, onConfirm func(selectedUsers []string)) *UserTagModal {
	var members []*store.AutocompleteMemberEntry
	if room != nil {
		members = room.GetMembers()
	}
	return NewUserTagModal(parent, members, onConfirm)
}

// Focus delegates focus to container and search input.
func (m *UserTagModal) Focus() {
	m.inputLock.Lock()
	if m.container != nil {
		m.container.Focus()
	}
	if m.search != nil {
		m.search.Focus()
	}
	m.inputLock.Unlock()
}

// Blur blurs search input and container.
func (m *UserTagModal) Blur() {
	m.inputLock.Lock()
	if m.search != nil {
		m.search.Blur()
	}
	if m.container != nil {
		m.container.Blur()
	}
	m.inputLock.Unlock()
}

// renderListLocked renders up to 100 candidate rows with [x] / [ ] indicators and escaped display names into results TextView.
// Caller must hold m.lock.
func (m *UserTagModal) renderListLocked() {
	var plainBuf strings.Builder
	m.results.Clear()
	limit := len(m.matches)
	if limit > 100 {
		limit = 100
	}
	for i := 0; i < limit; i++ {
		entry := m.matches[i]
		if entry == nil {
			continue
		}
		checked := " "
		if _, ok := m.selectedUsers[entry.UserID]; ok {
			checked = "x"
		}
		indicator := fmt.Sprintf("[%s] ", checked)
		displayName := entry.Displayname
		if displayName == "" {
			displayName = entry.UserID.Localpart()
		}
		if displayName == "" {
			displayName = string(entry.UserID)
		}
		escapedName := mauview.Escape(displayName)
		userIDStr := string(entry.UserID)
		if !strings.HasPrefix(userIDStr, "@") && userIDStr != "" {
			userIDStr = "@" + userIDStr
		}
		escapedUserID := mauview.Escape(userIDStr)
		var rowText string
		if userIDStr != "" {
			rowText = fmt.Sprintf("%s%s (%s)", indicator, escapedName, escapedUserID)
		} else {
			rowText = fmt.Sprintf("%s%s", indicator, escapedName)
		}
		plainBuf.WriteString(rowText + "\n")
		_, _ = fmt.Fprintf(m.results, `["%d"]%s[""]%s`, i, rowText, "\n")
	}
	m.renderedText = plainBuf.String()
	if limit > 0 && m.selected < limit {
		m.results.Highlight(strconv.Itoa(m.selected))
		m.results.ScrollToHighlight()
	} else {
		m.results.Highlight()
	}
}

// renderList acquires the lock and renders the candidate list.
func (m *UserTagModal) renderList() {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.renderListLocked()
}

// RenderList is the exported version of renderList.
func (m *UserTagModal) RenderList() {
	m.renderList()
}

// filterLocked filters room members by query string, preserving existing selections.
// Caller must hold m.lock.
func (m *UserTagModal) filterLocked(query string) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		m.matches = make([]*store.AutocompleteMemberEntry, 0, len(m.members))
		for _, mem := range m.members {
			if mem != nil {
				m.matches = append(m.matches, mem)
			}
		}
	} else {
		m.matches = make([]*store.AutocompleteMemberEntry, 0)
		for _, member := range m.members {
			if member == nil {
				continue
			}
			searchTarget := strings.ToLower(member.Displayname + " " + string(member.UserID) + " " + member.SearchString)
			if strings.Contains(searchTarget, q) ||
				fuzzy.MatchFold(q, member.Displayname) ||
				fuzzy.MatchFold(q, string(member.UserID)) ||
				fuzzy.MatchFold(q, member.SearchString) {
				m.matches = append(m.matches, member)
			}
		}
	}
	numItems := len(m.matches)
	if numItems > 100 {
		numItems = 100
	}
	if numItems == 0 {
		m.selected = 0
	} else if m.selected >= numItems {
		m.selected = 0
	}
	m.renderListLocked()
}

// filter acquires the lock and filters room members by query string.
func (m *UserTagModal) filter(query string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.filterLocked(query)
}

// Filter is the exported version of filter.
func (m *UserTagModal) Filter(query string) {
	m.filter(query)
}

// changeHandler handles query changes (conforming to FuzzyPickerModal pattern).
func (m *UserTagModal) changeHandler(str string) {
	m.filter(str)
}

// ChangeHandler is the exported version of changeHandler.
func (m *UserTagModal) ChangeHandler(str string) {
	m.changeHandler(str)
}

// toggleCurrentLocked toggles the selection checkbox on the highlighted candidate and advances highlight.
// Caller must hold m.lock.
func (m *UserTagModal) toggleCurrentLocked() {
	numItems := len(m.matches)
	if numItems > 100 {
		numItems = 100
	}
	if numItems == 0 || m.selected >= numItems {
		return
	}
	current := m.matches[m.selected]
	if current == nil {
		return
	}
	if _, exists := m.selectedUsers[current.UserID]; exists {
		delete(m.selectedUsers, current.UserID)
		newOrder := make([]id.UserID, 0, len(m.selectedOrder))
		for _, uid := range m.selectedOrder {
			if uid != current.UserID {
				newOrder = append(newOrder, uid)
			}
		}
		m.selectedOrder = newOrder
	} else {
		m.selectedUsers[current.UserID] = struct{}{}
		m.selectedOrder = append(m.selectedOrder, current.UserID)
	}
	if numItems > 0 {
		m.selected = (m.selected + 1) % numItems
	}
	m.renderListLocked()
}

// toggleCurrent acquires the lock and toggles the selection checkbox on the highlighted candidate.
func (m *UserTagModal) toggleCurrent() {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.toggleCurrentLocked()
}

// ToggleCurrent is the exported version of toggleCurrent.
func (m *UserTagModal) ToggleCurrent() {
	m.toggleCurrent()
}

// confirm handles dual confirmation and modal closure with composer focus restoration.
// Follows open-call discipline: extracts state under lock, releases lock, then calls external methods.
func (m *UserTagModal) confirm() {
	m.lock.Lock()
	if m.dismissed {
		m.lock.Unlock()
		return
	}
	m.dismissed = true

	var tags []string
	if len(m.selectedOrder) > 0 {
		lookup := make(map[id.UserID]*store.AutocompleteMemberEntry)
		for _, mem := range m.members {
			if mem != nil {
				lookup[mem.UserID] = mem
			}
		}
		for _, uid := range m.selectedOrder {
			if entry, ok := lookup[uid]; ok && entry != nil {
				name := entry.Displayname
				if name == "" {
					name = entry.UserID.Localpart()
				}
				if name == "" {
					name = string(entry.UserID)
				}
				tags = append(tags, name)
			} else {
				tags = append(tags, string(uid))
			}
		}
	} else if len(m.matches) > 0 && m.selected < len(m.matches) {
		highlighted := m.matches[m.selected]
		if highlighted != nil {
			name := highlighted.Displayname
			if name == "" {
				name = highlighted.UserID.Localpart()
			}
			if name == "" {
				name = string(highlighted.UserID)
			}
			tags = append(tags, name)
		}
	}

	parent := m.parent
	onConfirm := m.onConfirm
	m.lock.Unlock()

	if parent != nil {
		parent.HideModal()
		if parent.currentRoom != nil && parent.currentRoom.input != nil {
			parent.currentRoom.Focus()
			parent.currentRoom.input.Focus()
		}
	}
	if onConfirm != nil {
		onConfirm(tags)
	}
}

// Confirm is the exported version of confirm.
func (m *UserTagModal) Confirm() {
	m.confirm()
}

// cancel cancels modal without invoking onConfirm and restores composer focus.
// Follows open-call discipline: sets dismissed under lock, releases lock, then calls external methods.
func (m *UserTagModal) cancel() {
	m.lock.Lock()
	if m.dismissed {
		m.lock.Unlock()
		return
	}
	m.dismissed = true
	parent := m.parent
	m.lock.Unlock()

	if parent != nil {
		parent.HideModal()
		if parent.currentRoom != nil && parent.currentRoom.input != nil {
			parent.currentRoom.Focus()
			parent.currentRoom.input.Focus()
		}
	}
}

// Cancel is the exported version of cancel.
func (m *UserTagModal) Cancel() {
	m.cancel()
}

// OnKeyEvent handles keyboard navigation, multi-select toggling via Tab, dual confirmation on Enter,
// and cancellation on Escape. Printable runes fall through to search InputArea.
// Returns false immediately if dismissed so keystrokes fall through to the active view.
func (m *UserTagModal) OnKeyEvent(event mauview.KeyEvent) bool {
	m.lock.RLock()
	if m.dismissed {
		m.lock.RUnlock()
		return false
	}
	m.lock.RUnlock()

	switch event.Key() {
	case tcell.KeyTab:
		m.toggleCurrent()
		return true
	case tcell.KeyDown:
		m.lock.Lock()
		numItems := len(m.matches)
		if numItems > 100 {
			numItems = 100
		}
		if numItems > 0 {
			m.selected = (m.selected + 1) % numItems
			m.renderListLocked()
		}
		m.lock.Unlock()
		return true
	case tcell.KeyUp:
		m.lock.Lock()
		numItems := len(m.matches)
		if numItems > 100 {
			numItems = 100
		}
		if numItems > 0 {
			m.selected = (m.selected - 1 + numItems) % numItems
			m.renderListLocked()
		}
		m.lock.Unlock()
		return true
	case tcell.KeyBacktab:
		m.lock.Lock()
		numItems := len(m.matches)
		if numItems > 100 {
			numItems = 100
		}
		if numItems > 0 {
			m.selected = (m.selected - 1 + numItems) % numItems
			m.renderListLocked()
		}
		m.lock.Unlock()
		return true
	case tcell.KeyEnter:
		m.confirm()
		return true
	case tcell.KeyEscape:
		m.cancel()
		return true
	case tcell.KeyCtrlA:
		m.inputLock.Lock()
		m.search.SetCursorOffset(0)
		m.inputLock.Unlock()
		return true
	default:
		m.inputLock.Lock()
		handled := m.search.OnKeyEvent(event)
		text := m.search.GetText()
		m.inputLock.Unlock()
		m.filter(text)
		return handled
	}
}

// OnPasteEvent handles paste events safely under inputLock.
func (m *UserTagModal) OnPasteEvent(event mauview.PasteEvent) bool {
	m.inputLock.Lock()
	if m.search == nil {
		m.inputLock.Unlock()
		return false
	}
	handled := m.search.OnPasteEvent(event)
	text := m.search.GetText()
	m.inputLock.Unlock()
	m.filter(text)
	return handled
}

// GetSelectedUsers returns a copy of the map of selected user IDs.
func (m *UserTagModal) GetSelectedUsers() map[id.UserID]struct{} {
	m.lock.RLock()
	defer m.lock.RUnlock()
	res := make(map[id.UserID]struct{}, len(m.selectedUsers))
	for k, v := range m.selectedUsers {
		res[k] = v
	}
	return res
}

// GetSelectedOrder returns a copy of the slice of selected user IDs in selection order.
func (m *UserTagModal) GetSelectedOrder() []id.UserID {
	m.lock.RLock()
	defer m.lock.RUnlock()
	res := make([]id.UserID, len(m.selectedOrder))
	copy(res, m.selectedOrder)
	return res
}

// GetMatches returns a copy of the currently matched candidates.
func (m *UserTagModal) GetMatches() []*store.AutocompleteMemberEntry {
	m.lock.RLock()
	defer m.lock.RUnlock()
	res := make([]*store.AutocompleteMemberEntry, len(m.matches))
	copy(res, m.matches)
	return res
}

// GetSelectedIndex returns the currently highlighted index.
func (m *UserTagModal) GetSelectedIndex() int {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.selected
}

// GetRenderedText returns the rendered plain text buffer for testing and verification.
func (m *UserTagModal) GetRenderedText() string {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.renderedText
}

// IsDismissed returns true if the modal has been closed via confirm or cancel.
func (m *UserTagModal) IsDismissed() bool {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.dismissed
}

// SearchInput returns the search InputArea widget.
func (m *UserTagModal) SearchInput() *mauview.InputArea {
	m.inputLock.Lock()
	defer m.inputLock.Unlock()
	return m.search
}

// GetSearchText returns the current text in the search input area safely under inputLock.
func (m *UserTagModal) GetSearchText() string {
	m.inputLock.Lock()
	defer m.inputLock.Unlock()
	if m.search == nil {
		return ""
	}
	return m.search.GetText()
}

// ResultsView returns the results TextView widget.
func (m *UserTagModal) ResultsView() *mauview.TextView {
	return m.results
}
