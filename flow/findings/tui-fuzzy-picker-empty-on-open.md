# Finding: TUI Fuzzy Picker Modal Blank on Open (Empty Query)

## Symptom
When opening the **Space Switcher** (`Ctrl+s` / `Alt+s`) or **Room Switcher** (`Ctrl+k` / `Alt+k`), the modal displays an empty/blank result area:

```
╔═════════════Space Switcher═════════════╗
║                                        ║
║                                        ║
║                                        ║
║                                        ║
║                                        ║
╚════════════════════════════════════════╝
```

The user sees zero items until they actively type letters into the search input.

---

## Root Cause Analysis
- **File:** `tui/fuzzy-picker-modal.go` (`changeHandler`)
- **Code:**
  ```go
  func (fp *FuzzyPickerModal) changeHandler(str string) {
      fp.matches = fuzzy.RankFindFold(str, fp.titles)
      if len(str) > 0 && len(fp.matches) > 0 {
          sort.Sort(fp.matches)
          fp.results.Clear()
          for _, match := range fp.matches {
              _, _ = fmt.Fprintf(fp.results, `["%d"]%s[""]%s`, match.OriginalIndex, match.Target, "\n")
          }
          fp.results.Highlight(strconv.Itoa(fp.matches[0].OriginalIndex))
          fp.selected = 0
          fp.results.ScrollToBeginning()
      } else {
          fp.results.Clear()
          fp.results.Highlight()
      }
  }
  ```
- **Bug:** `if len(str) > 0` explicitly branches to `fp.results.Clear()` when `str == ""`. Because modals initialize with an empty query (`""`), the result box is completely cleared, presenting a blank UI.

---

## Expected UX Behavior
1. **Immediate Full List:** When the modal opens with query `""`, it must immediately list all available options (`fp.titles`) in their original order.
2. **Default Selection:** The first entry (e.g. `"All Rooms"` in the Space Switcher) should be highlighted by default so pressing `Enter` immediately selects it.
3. **Fuzzy Filter on Typing:** As soon as the user types characters (`len(str) > 0`), it filters and sorts by fuzzy score. If the query is erased back to empty (`""`), it returns to showing the full list.

---

## Required Fix
In `tui/fuzzy-picker-modal.go`:
1. When `len(str) == 0`:
   - Fill `fp.matches` with all `fp.titles` indexed `0..len(titles)-1`.
   - Render all titles into `fp.results`.
   - Highlight index `0` and scroll to beginning.
2. In `NewFuzzyPickerModal`:
   - Call `fp.changeHandler("")` immediately after constructing `search` and `results` to populate the initial list on open.
