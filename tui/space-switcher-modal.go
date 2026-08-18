// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2020 Tulir Asokan
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
	"sort"
	"strconv"

	"github.com/gdamore/tcell/v2"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"go.mau.fi/mauview"

	"go.mau.fi/gomuks/pkg/rpc/store"
	"go.mau.fi/gomuks/tui/config"
	"go.mau.fi/gomuks/tui/debug"
)

type SpaceSwitcherModal struct {
	mauview.Component

	container *mauview.Box

	search  *mauview.InputArea
	results *mauview.TextView

	matches  fuzzy.Ranks
	selected int

	spaceList   []*store.SpaceEntry
	spaceTitles []string

	parent *MainView
}

func NewSpaceSwitcherModal(mainView *MainView, width int, height int) *SpaceSwitcherModal {
	ss := &SpaceSwitcherModal{
		parent:    mainView,
		spaceList: mainView.matrix.GetSpaceList(),
	}
	ss.spaceTitles = make([]string, len(ss.spaceList))
	for i, space := range ss.spaceList {
		ss.spaceTitles[i] = space.Name
	}

	ss.results = mauview.NewTextView().SetRegions(true)
	ss.search = mauview.NewInputArea().
		SetChangedFunc(ss.changeHandler).
		SetTextColor(tcell.ColorWhite).
		SetBackgroundColor(tcell.ColorDarkCyan)
	ss.search.Focus()

	flex := mauview.NewFlex().
		SetDirection(mauview.FlexRow).
		AddFixedComponent(ss.search, 1).
		AddProportionalComponent(ss.results, 1)

	ss.container = mauview.NewBox(flex).
		SetBorder(true).
		SetTitle("Space Switcher").
		SetBlurCaptureFunc(func() bool {
			ss.parent.HideModal()
			return true
		})

	ss.Component = mauview.Center(ss.container, width, height).SetAlwaysFocusChild(true)

	return ss
}

func (ss *SpaceSwitcherModal) Focus() {
	ss.container.Focus()
}

func (ss *SpaceSwitcherModal) Blur() {
	ss.container.Blur()
}

func (ss *SpaceSwitcherModal) changeHandler(str string) {
	ss.matches = fuzzy.RankFindFold(str, ss.spaceTitles)
	if len(str) > 0 && len(ss.matches) > 0 {
		sort.Sort(ss.matches)
		ss.results.Clear()
		for _, match := range ss.matches {
			_, _ = fmt.Fprintf(ss.results, `["%d"]%s[""]%s`, match.OriginalIndex, match.Target, "\n")
		}
		ss.results.Highlight(strconv.Itoa(ss.matches[0].OriginalIndex))
		ss.selected = 0
		ss.results.ScrollToBeginning()
	} else {
		ss.results.Clear()
		ss.results.Highlight()
	}
}

func (ss *SpaceSwitcherModal) OnKeyEvent(event mauview.KeyEvent) bool {
	highlights := ss.results.GetHighlights()
	kb := config.Keybind{
		Key: event.Key(),
		Ch:  event.Rune(),
		Mod: event.Modifiers(),
	}
	switch ss.parent.config.Keybindings.Modal[kb] {
	case "cancel":
		ss.parent.HideModal()
		return true
	case "select_next":
		if len(highlights) > 0 {
			ss.selected = (ss.selected + 1) % len(ss.matches)
			ss.results.Highlight(strconv.Itoa(ss.matches[ss.selected].OriginalIndex))
			ss.results.ScrollToHighlight()
		}
		return true
	case "select_prev":
		if len(highlights) > 0 {
			ss.selected = (ss.selected - 1) % len(ss.matches)
			if ss.selected < 0 {
				ss.selected += len(ss.matches)
			}
			ss.results.Highlight(strconv.Itoa(ss.matches[ss.selected].OriginalIndex))
			ss.results.ScrollToHighlight()
		}
		return true
	case "confirm":
		if len(highlights) > 0 {
			selectedSpace := ss.spaceList[ss.matches[ss.selected].OriginalIndex]
			debug.Print("Space Switcher: Selected", selectedSpace.Name, selectedSpace.RoomID)
			ss.parent.SwitchToSpace(selectedSpace.RoomID)
		}
		ss.parent.HideModal()
		ss.results.Clear()
		ss.search.SetText("")
		return true
	}
	return ss.search.OnKeyEvent(event)
}
