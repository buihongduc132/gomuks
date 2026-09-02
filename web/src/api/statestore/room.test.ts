// gomuks - A Matrix client written in Go.
// Copyright (C) 2024 Tulir Asokan
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
import { beforeEach, describe, expect, test, vi } from "vitest"
import {
	DBRoom,
	RawDBEvent,
	RoomNameQuality,
	SyncRoom,
	UnreadType,
} from "../types"
import { RoomStateStore } from "./room.ts"
import type { StateStore } from "./main.ts"

// The room preference cache persists to localStorage — reset between tests.
beforeEach(() => {
	localStorage.clear()
})

const USER = "@me:example.com"

function makeMeta(overrides: Partial<DBRoom> = {}): DBRoom {
	return {
		room_id: "!room:example.com",
		name: "Test Room",
		name_quality: RoomNameQuality.Explicit,
		explicit_avatar: false,
		has_member_list: false,
		preview_event_rowid: 0,
		sorting_timestamp: 1000,
		unread_highlights: 0,
		unread_notifications: 0,
		unread_messages: 0,
		marked_unread: false,
		...overrides,
	}
}

function makeEvent(overrides: Partial<RawDBEvent> = {}): RawDBEvent {
	return {
		rowid: 1,
		timeline_rowid: 1,
		room_id: "!room:example.com",
		event_id: "$evt:example.com",
		sender: "@other:example.com",
		type: "m.room.message",
		timestamp: 1700000000000,
		content: { body: "hello" },
		unsigned: {},
		unread_type: UnreadType.None,
		...overrides,
	}
}

function makeParent(): StateStore {
	return {
		userID: USER,
		widgetListeners: new Set(),
		stateCache: null,
		invalidateEmojiPacksCache: vi.fn(),
	} as unknown as StateStore
}

describe("RoomStateStore basics", () => {
	test("constructor initializes from meta", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		expect(room.roomID).toBe("!room:example.com")
		expect(room.meta.current.name).toBe("Test Room")
		expect(room.timeline).toEqual([])
		expect(room.typing).toEqual([])
	})

	test("searchString is computed from meta", () => {
		const room = new RoomStateStore(
			makeMeta({ name: "Searchable", canonical_alias: "#test:example.com" }),
			makeParent(),
		)
		expect(room.searchString).toContain("searchable")
	})
})

describe("applyEvent", () => {
	test("applies basic event", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const evt = makeEvent()
		room.applyEvent(evt)
		expect(room.eventsByRowID.get(1)).toBeDefined()
		expect(room.eventsByID.get("$evt:example.com")).toBeDefined()
	})

	test("sets mem and pending flags", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const evt = makeEvent()
		room.applyEvent(evt, true)
		const memEvt = room.eventsByRowID.get(1)!
		expect(memEvt.mem).toBe(true)
		expect(memEvt.pending).toBe(true)
	})

	test("handles encrypted events", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const evt = makeEvent({
			type: "m.room.encrypted",
			decrypted: { body: "decrypted body" },
			decrypted_type: "m.room.message",
		})
		room.applyEvent(evt)
		const memEvt = room.eventsByRowID.get(1)!
		expect(memEvt.type).toBe("m.room.message")
		expect(memEvt.encrypted).toBeDefined()
	})

	test("applies edit to target when target has last_edit_rowid", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const original = makeEvent({ rowid: 1, content: { body: "original" } })
		room.applyEvent(original)
		const edit = makeEvent({
			rowid: 2,
			event_id: "$edit:example.com",
			relation_type: "m.replace",
			relates_to: "$evt:example.com",
			content: { "m.new_content": { body: "edited" } },
		})
		// Edit alone does not retroactively modify the target (target.last_edit_rowid not yet set)
		room.applyEvent(edit)
		expect(room.eventsByRowID.get(1)!.content.body).toBe("original")
		// Re-apply the target with last_edit_rowid pointing at the edit
		const originalWithEdit = makeEvent({
			rowid: 1,
			content: { body: "original" },
			last_edit_rowid: 2,
		})
		room.applyEvent(originalWithEdit)
		const memEvt = room.eventsByRowID.get(1)!
		expect(memEvt.content.body).toBe("edited")
		expect(memEvt.last_edit).toBeDefined()
		expect(memEvt.orig_content).toEqual({ body: "original" })
	})

	test("applies edits via m.replace when target's last_edit_rowid matches", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		// Target applied first WITH last_edit_rowid pointing at the (not yet applied) edit
		const original = makeEvent({ rowid: 1, content: { body: "original" }, last_edit_rowid: 2 })
		room.applyEvent(original)
		// last_edit_rowid is set but the edit event isn't applied yet → no last_edit
		expect(room.eventsByRowID.get(1)!.content.body).toBe("original")
		const edit = makeEvent({
			rowid: 2,
			event_id: "$edit:example.com",
			relation_type: "m.replace",
			relates_to: "$evt:example.com",
			content: { "m.new_content": { body: "edited via replace" } },
		})
		room.applyEvent(edit)
		const memEvt = room.eventsByRowID.get(1)!
		expect(memEvt.content.body).toBe("edited via replace")
		expect(memEvt.last_edit?.rowid).toBe(2)
		expect(memEvt.orig_content).toEqual({ body: "original" })
	})

	test("removes pending event from pendingEvents when non-pending applied", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const evt = makeEvent({ rowid: 5 })
		room.applyEvent(evt, true)
		room.pendingEvents.push(5)
		room.applyEvent(makeEvent({ rowid: 5 }), false)
		expect(room.pendingEvents).not.toContain(5)
	})
})

describe("applySync", () => {
	test("applies meta changes", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.applySync(
			{ meta: makeMeta({ name: "New Name", sorting_timestamp: 2000 }) } as SyncRoom,
			false,
		)
		expect(room.meta.current.name).toBe("New Name")
		expect(room.meta.current.sorting_timestamp).toBe(2000)
	})

	test("applies events and timeline", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const evt = makeEvent()
		room.applySync(
			{
				events: [evt],
				timeline: [{ timeline_rowid: 1, event_rowid: 1 }],
			} as SyncRoom,
			false,
		)
		expect(room.eventsByRowID.get(1)).toBeDefined()
		expect(room.timeline.length).toBe(1)
	})

	test("applies state events", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const stateEvt = makeEvent({
			rowid: 10,
			event_id: "$state:example.com",
			type: "m.room.name",
			state_key: "",
			content: { name: "New Name" },
		})
		room.applySync(
			{
				events: [stateEvt],
				state: { "m.room.name": { "": 10 } },
			} as SyncRoom,
			false,
		)
		expect(room.state.get("m.room.name")?.get("")).toBe(10)
	})

	test("reset clears timeline", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.timeline = [{ timeline_rowid: 1, event_rowid: 1 }]
		room.pendingEvents.push(1)
		room.applySync({ reset: true } as SyncRoom, false)
		expect(room.timeline).toEqual([])
		expect(room.pendingEvents).toEqual([])
	})

	test("timeline appended when not reset", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.timeline = [{ timeline_rowid: 1, event_rowid: 1 }]
		room.applySync({ timeline: [{ timeline_rowid: 2, event_rowid: 2 }] } as SyncRoom, false)
		expect(room.timeline.length).toBe(2)
	})

	test("account data is stored", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.applySync(
			{
				account_data: {
					"m.tag": {
						user_id: USER,
						room_id: "!room:example.com",
						type: "m.tag",
						content: { tags: { "m.favourite": {} } },
					},
				},
			} as SyncRoom,
			false,
		)
		expect(room.accountData.get("m.tag")).toBeDefined()
	})

	test("unread notifications zero clears open notifications", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const mockNotif = { close: vi.fn() }
		room.openNotifications.set(1, mockNotif as unknown as Notification)
		room.applySync(
			{
				meta: makeMeta({ unread_notifications: 0, unread_highlights: 0 }),
			} as SyncRoom,
			false,
		)
		expect(mockNotif.close).toHaveBeenCalled()
		expect(room.openNotifications.size).toBe(0)
	})
})

describe("applyFullState", () => {
	test("applies full state and sets stateLoaded", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const state = [
			makeEvent({ rowid: 1, type: "m.room.name", state_key: "", content: { name: "Test" } }),
			makeEvent({ rowid: 2, type: "m.room.member", state_key: "@user:example.com", content: { membership: "join" } }),
		]
		room.applyFullState(state, false)
		expect(room.stateLoaded).toBe(true)
		expect(room.fullMembersLoaded).toBe(true)
		expect(room.state.get("m.room.name")?.get("")).toBe(1)
		expect(room.state.get("m.room.member")?.get("@user:example.com")).toBe(2)
	})

	test("omitMembers keeps existing member state", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.state.set("m.room.member", new Map([["@old:example.com", 99]]))
		const state = [
			makeEvent({ rowid: 1, type: "m.room.name", state_key: "", content: { name: "Test" } }),
		]
		room.applyFullState(state, true)
		expect(room.state.get("m.room.member")?.get("@old:example.com")).toBe(99)
		expect(room.fullMembersLoaded).toBe(false)
	})
})

describe("applyState", () => {
	test("applies single state event", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const evt = makeEvent({
			rowid: 5,
			type: "m.room.topic",
			state_key: "",
			content: { topic: "A topic" },
		})
		room.applyState(evt)
		expect(room.state.get("m.room.topic")?.get("")).toBe(5)
	})

	test("throws if state_key is undefined", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const evt = makeEvent({ type: "m.room.name" })
		delete evt.state_key
		expect(() => room.applyState(evt)).toThrow("missing state key")
	})
})

describe("getStateEvent", () => {
	test("returns state event by type and state_key", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const evt = makeEvent({ rowid: 1, type: "m.room.name", state_key: "", content: { name: "Test" } })
		room.applyState(evt)
		expect(room.getStateEvent("m.room.name", "")).toBeDefined()
		expect(room.getStateEvent("m.room.name", "")!.content.name).toBe("Test")
	})

	test("returns fake gomuks member event", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const evt = room.getStateEvent("m.room.member", "@gomuks")
		expect(evt).toBeDefined()
		expect(evt!.sender).toBe("@gomuks")
	})

	test("returns undefined for unknown state", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		expect(room.getStateEvent("m.room.name", "")).toBeUndefined()
	})
})

describe("getMembers", () => {
	test("returns empty when no members", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		expect(room.getMembers()).toEqual([])
	})

	test("returns joined members sorted by power level", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.applyState(makeEvent({
			rowid: 1, type: "m.room.member", state_key: "@admin:example.com",
			content: { membership: "join", displayname: "Admin" },
		}))
		room.applyState(makeEvent({
			rowid: 2, type: "m.room.member", state_key: "@user:example.com",
			content: { membership: "join", displayname: "User" },
		}))
		room.applyState(makeEvent({
			rowid: 3, type: "m.room.power_levels", state_key: "",
			content: { users: { "@admin:example.com": 100 } },
		}))
		const members = room.getMembers()
		expect(members.length).toBe(2)
		expect(members[0].userID).toBe("@admin:example.com")
	})

	test("filters out left members", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.applyState(makeEvent({
			rowid: 1, type: "m.room.member", state_key: "@left:example.com",
			content: { membership: "leave" },
		}))
		expect(room.getMembers()).toEqual([])
	})
})

describe("getViaServers", () => {
	test("returns own server", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const vias = room.getViaServers()
		expect(vias).toContain("example.com")
	})

	test("includes server of highest power user", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.applyState(makeEvent({
			rowid: 1, type: "m.room.member", state_key: "@admin:other.com",
			content: { membership: "join" },
		}))
		room.applyState(makeEvent({
			rowid: 2, type: "m.room.power_levels", state_key: "",
			content: { users: { "@admin:other.com": 100 } },
		}))
		const vias = room.getViaServers()
		expect(vias).toContain("other.com")
	})
})

describe("getPinnedEvents", () => {
	test("returns empty when no pinned events", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		expect(room.getPinnedEvents()).toEqual([])
	})

	test("returns pinned event IDs", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.applyState(makeEvent({
			rowid: 1, type: "m.room.pinned_events", state_key: "",
			content: { pinned: ["$evt1:example.com", "$evt2:example.com"] },
		}))
		expect(room.getPinnedEvents()).toEqual(["$evt1:example.com", "$evt2:example.com"])
	})

	test("filters non-string pinned events", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.applyState(makeEvent({
			rowid: 1, type: "m.room.pinned_events", state_key: "",
			content: { pinned: ["$evt1:example.com", 123, null] },
		}))
		expect(room.getPinnedEvents()).toEqual(["$evt1:example.com"])
	})
})

describe("applyReceipts", () => {
	test("applies receipts to event", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const evt = makeEvent({ rowid: 1, event_id: "$evt:example.com" })
		room.applyEvent(evt)
		room.applyReceipts(
			[{ user_id: "@other:example.com", receipt_type: "m.read", event_id: "$evt:example.com", timestamp: 1000 }],
			"$evt:example.com",
			false,
		)
		expect(room.receiptsByEventID.get("$evt:example.com")).toBeDefined()
		expect(room.receiptsByUserID.get("@other:example.com")).toBeDefined()
	})

	test("skips receipts for unknown events", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.applyReceipts(
			[{ user_id: "@other:example.com", receipt_type: "m.read", event_id: "$unknown:example.com", timestamp: 1000 }],
			"$unknown:example.com",
			false,
		)
		expect(room.receiptsByEventID.size).toBe(0)
	})
})

describe("applyPagination", () => {
	test("applies pagination events", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const history = [
			makeEvent({ rowid: 1, timeline_rowid: 1 }),
			makeEvent({ rowid: 2, timeline_rowid: 2 }),
		]
		room.applyPagination(history, [], {})
		expect(room.timeline.length).toBe(2)
		expect(room.eventsByRowID.size).toBe(2)
	})

	test("reset replaces timeline", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.timeline = [{ timeline_rowid: 99, event_rowid: 99 }]
		const history = [makeEvent({ rowid: 1, timeline_rowid: 1 })]
		room.applyPagination(history, [], {}, true)
		expect(room.timeline.length).toBe(1)
		expect(room.timeline[0].event_rowid).toBe(1)
	})
})

describe("applyDecrypted", () => {
	test("applies decrypted events", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const evt = makeEvent({ rowid: 1, type: "m.room.encrypted" })
		room.applyEvent(evt)
		room.applyDecrypted({
			room_id: "!room:example.com",
			events: [makeEvent({
				rowid: 1,
				type: "m.room.encrypted",
				decrypted: { body: "decrypted" },
				decrypted_type: "m.room.message",
			})],
		})
		expect(room.eventsByRowID.get(1)!.content.body).toBe("decrypted")
	})

	test("updates preview event rowid", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.applyDecrypted({
			room_id: "!room:example.com",
			preview_event_rowid: 5,
			events: [],
		})
		expect(room.meta.current.preview_event_rowid).toBe(5)
	})
})

describe("applyTyping / clearTyping", () => {
	test("applyTyping updates typing list", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.applyTyping(["@a:example.com", "@b:example.com"])
		expect(room.typing).toEqual(["@a:example.com", "@b:example.com"])
	})

	test("clearTyping clears when non-empty", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.applyTyping(["@a:example.com"])
		room.clearTyping()
		expect(room.typing).toEqual([])
	})

	test("clearTyping does nothing when empty", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.clearTyping()
		expect(room.typing).toEqual([])
	})
})

describe("doGarbageCollection", () => {
	test("keeps preview event and its sender", () => {
		const room = new RoomStateStore(makeMeta({ preview_event_rowid: 1 }), makeParent())
		room.applyEvent(makeEvent({ rowid: 1, sender: "@sender:example.com" }))
		room.applyEvent(makeEvent({ rowid: 2, type: "m.room.member", state_key: "@other:example.com" }))
		const [deletedEvents, deletedState] = room.doGarbageCollection()
		expect(deletedEvents).toBe(1)
		expect(room.eventsByRowID.has(1)).toBe(true)
		expect(room.eventsByRowID.has(2)).toBe(false)
	})
})

describe("removeFailedEvent", () => {
	test("removes pending event", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const evt = makeEvent({ rowid: 5 })
		room.applyEvent(evt, true)
		room.pendingEvents.push(5)
		room.removeFailedEvent(room.eventsByRowID.get(5)!)
		expect(room.pendingEvents).not.toContain(5)
	})
})

describe("subscribeThread", () => {
	test("subscribes and unsubscribes", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const listener = vi.fn()
		const unsub = room.subscribeThread("$root:example.com", listener)
		unsub()
		// After unsubscribe, listener should not be called
		expect(listener).not.toHaveBeenCalled()
	})
})

describe("invalidateStateCaches", () => {
	test("invalidates emoji pack cache on image pack change", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.invalidateStateCaches("m.room.image_pack", "")
		// Should not throw
	})

	test("invalidates member cache on member change", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.invalidateStateCaches("m.room.member", "@user:example.com")
		// Should not throw
	})

	test("invalidates member cache on power level change", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.invalidateStateCaches("m.room.power_levels", "")
		// Should not throw
	})
})

describe("getStateForCache", () => {
	test("returns state for cache with preview event", () => {
		const room = new RoomStateStore(makeMeta({ preview_event_rowid: 1 }), makeParent())
		room.applyEvent(makeEvent({ rowid: 1, sender: "@sender:example.com" }))
		const state = room.getStateForCache()
		expect(state.meta).toBeDefined()
		expect(state.events.length).toBeGreaterThan(0)
	})
})

describe("setViewingRedacted", () => {
	test("sets viewing_redacted flag", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const evt = makeEvent({ rowid: 1, redacted_by: "$redact:example.com" })
		room.applyEvent(evt)
		const memEvt = room.eventsByRowID.get(1)!
		room.setViewingRedacted(memEvt, true)
		expect(room.eventsByRowID.get(1)!.viewing_redacted).toBe(true)
	})
})
