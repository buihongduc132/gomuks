# Explore Turn 1 — user-requests
> Date: 2026-09-06
> Phase: explore
> Status: completed

> **[user]** (verbatim — trust chain L1)
> /10-ospx-explore 1. whenever the image is downloading , showing a place holder for it ; 2. having the flag to turn of image auto downloaded ; 3. whenever reply , make it to have preview of the message will be reply instead of just the message id; 4. find out how to be able to tag user as well (could implement it as a modal popup like the search room / search spaces , could tag multiple user as well

[assistant]
> "1. whenever the image is downloading , showing a place holder for it ;"

**Web Client**: This is already partially implemented. In `web/src/ui/timeline/content/MediaMessageBody.tsx`, it uses `react-blurhash` (if the server provides a blurhash) or a `<GridLoader>` spinner as a placeholder while the media fetches. If you meant a placeholder during *upload* (before the message hits the server), the timeline doesn't currently render a skeleton—the composer just holds the attachment until sent. 
**TUI Client**: Images don't auto-download right now at all (`DownloadPreview()` is completely commented out in `tui/messages/filemessage.go`), so there is no downloading state to show a placeholder for.

> "2. having the flag to turn of image auto downloaded ;"

**Web Client**: This flag already exists. It is the `show_media_previews` preference (defined in `web/src/api/types/preferences/preferences.ts`).
**TUI Client**: This also already exists via the `DisableDownloads` and `DisableImages` flags in `tui/config/config.go`.

> "3. whenever reply , make it to have preview of the message will be reply instead of just the message id;"

**Web Client**: The Web client shows a rich preview *only* if the replied-to event is currently loaded in the local cache (`store.eventsByID`). If the event is not cached, `web/src/ui/timeline/ReplyBody.tsx` falls back to explicitly rendering `"Reply to unknown event"` and `<code>{eventID}</code>`. Because Gomuks actively strips the Matrix `<mx-reply>` fallback HTML (`RemoveReplyFallback()` in `pkg/hicli/sync.go`), it completely loses the preview context if the cache lookup fails.
**TUI Client**: The TUI HTML parser (`tui/messages/html/parser.go`) also explicitly returns `nil` for `<mx-reply>`, stripping the fallback. If it fails to find the event in memory, it draws absolutely nothing for the reply bubble. Furthermore, when *typing* a reply, the TUI just shows `Replying to @user - ` in the status bar (`tui/room-view.go`) instead of showing a preview snippet.

**Minimal Approach for #3**: We should stop stripping the `<mx-reply>` blockquote globally. If `eventID` fails to load, we can render the fallback HTML blockquote Matrix provides natively, so a text preview is always available.

> "4. find out how to be able to tag user as well (could implement it as a modal popup like the search room / search spaces , could tag multiple user as well"

**Web Client**: Currently, tagging relies strictly on inline typing (`@`) to trigger `web/src/ui/composer/Autocompleter.tsx`. To implement a modal popup identical to the "search room" (`CmdKDialog.tsx`), we would need to:
1. Create a new `UserTagModal` component mapped to a hotkey or composer button, rendered via `ModalContext`.
2. Query the room member list (similar to how `Autocompleter.tsx` filters users).
3. Allow multi-selection via a list of checkboxes or tokens.
4. On submit, iterate over the selections and inject `@Username` pills (and corresponding matrix URIs) into the `MessageComposer`'s text state.

Are you primarily focusing on the Web Client for these features? If you're ready to capture these into tasks, I can fast-forward this into a change via `/30a-ospx-ff`.

Status at end of turn 1: Awaiting user clarification on client (Web vs TUI).
