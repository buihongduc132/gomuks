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
import { RoomStateStore, fakeGomuksSender } from "./room.ts"
import type { StateStore } from "./main.ts"

const USER = "@me:example.com"

// The room preference cache persists to localStorage — reset between tests.
beforeEach(() => {
	localStorage.clear()
})

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

function makeParent(overrides: Record<string, unknown> = {}): StateStore {
	return {
		userID: USER,
		widgetListeners: new Set(),
		stateCache: null,
		invalidateEmojiPacksCache: vi.fn(),
		...overrides,
	} as unknown as StateStore
}

function imagePackEvent(stateKey: string, images: Record<string, unknown> | null) {
	return makeEvent({
		rowid: stateKey === "" ? 20 : 20 + stateKey.length,
		event_id: `$pack-${stateKey || "default"}:example.com`,
		type: "m.room.image_pack",
		state_key: stateKey,
		content: { pack: { display_name: `Pack ${stateKey}` }, images },
	} as Partial<RawDBEvent>)
}

describe("RoomStateStore additional coverage", () => {
	test("applySync with identical visible meta takes the non-emit path", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const meta = makeMeta()
		room.applySync({ meta } as SyncRoom, false)
		// otherMetaIsEqual true → metaChanged stays false, meta.current is replaced
		expect(room.meta.current).toBe(meta)
	})

	test("applySync with only non-visible meta change sets metaChanged", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.applySync({ meta: makeMeta({ sorting_timestamp: 5000 }) } as SyncRoom, false)
		expect(room.meta.current.sorting_timestamp).toBe(5000)
	})

	test("applySync with visible meta change emits new meta", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		let emitted = false
		room.meta.listen(() => {
			emitted = true
		})
		room.applySync({ meta: makeMeta({ name: "New Name" }) } as SyncRoom, false)
		expect(emitted).toBe(true)
		expect(room.meta.current.name).toBe("New Name")
		// toSearchableString lowercases and strips whitespace/homoglyphs
		expect(room.searchString).toContain("newna")
	})

	test("applySync turns empty dm_user_id into undefined", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const meta = makeMeta({ dm_user_id: "" })
		room.applySync({ meta } as SyncRoom, false)
		expect(meta.dm_user_id).toBeUndefined()
	})

	test("applySync lazy_load_summary equality paths", () => {
		const room = new RoomStateStore(makeMeta({
			lazy_load_summary: { "m.joined_member_count": 5, "m.heroes": ["@a:example.com", "@b:example.com"] },
		}), makeParent())
		// Same summary → visible meta equal
		room.applySync({ meta: makeMeta({
			sorting_timestamp: 2000,
			lazy_load_summary: { "m.joined_member_count": 5, "m.heroes": ["@a:example.com", "@b:example.com"] },
		}) } as SyncRoom, false)
		expect(room.meta.current.sorting_timestamp).toBe(2000)
		// Different heroes → visible meta differs → emit
		room.applySync({ meta: makeMeta({
			sorting_timestamp: 3000,
			lazy_load_summary: { "m.joined_member_count": 5, "m.heroes": ["@a:example.com", "@c:example.com"] },
		}) } as SyncRoom, false)
		expect(room.meta.current.sorting_timestamp).toBe(3000)
	})

	test("applySync without account data/events/state does nothing harmful", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.applySync({} as SyncRoom, false)
		expect(room.timeline).toEqual([])
	})

	test("getEmojiPack returns parsed pack", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.applyState(imagePackEvent("pack1", {
			smile: { url: "mxc://example.com/smile", body: "smiley" },
		}))
		const pack = room.getEmojiPack("pack1")
		expect(pack).not.toBeNull()
		expect(pack!.emojis.length).toBe(1)
		expect(pack!.emojis[0].n).toBe("smile")
	})

	test("getEmojiPack caches results", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.applyState(imagePackEvent("pack1", {
			smile: { url: "mxc://example.com/smile" },
		}))
		const pack1 = room.getEmojiPack("pack1")
		const pack2 = room.getEmojiPack("pack1")
		expect(pack1).toBe(pack2)
	})

	test("getEmojiPack returns null for missing pack", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		expect(room.getEmojiPack("nonexistent")).toBeNull()
	})

	test("getEmojiPack returns null for redacted pack", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const evt = imagePackEvent("pack1", { smile: { url: "mxc://x/y" } })
		evt.redacted_by = "$redaction:example.com"
		room.applyState(evt)
		expect(room.getEmojiPack("pack1")).toBeNull()
	})

	test("getEmojiPack returns null when images missing", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.applyState(imagePackEvent("pack1", null))
		expect(room.getEmojiPack("pack1")).toBeNull()
	})

	test("getEmojiPack falls back to legacy emotes type and uses room name for empty key", () => {
		const room = new RoomStateStore(makeMeta({ name: "My Room" }), makeParent())
		room.applyState(makeEvent({
			rowid: 31,
			event_id: "$legacy:example.com",
			type: "im.ponies.room_emotes",
			state_key: "",
			content: { pack: {}, images: { cat: { url: "mxc://example.com/cat" } } },
		}))
		const pack = room.getEmojiPack("")
		expect(pack).not.toBeNull()
	})

	test("getAllEmojiPacks returns all packs from both state types", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.applyState(imagePackEvent("pack1", { smile: { url: "mxc://example.com/smile" } }))
		room.applyState(makeEvent({
			rowid: 31,
			event_id: "$legacy:example.com",
			type: "im.ponies.room_emotes",
			state_key: "legacy",
			content: { pack: {}, images: { cat: { url: "mxc://example.com/cat" } } },
		}))
		const packs = room.getAllEmojiPacks()
		expect(Object.keys(packs).length).toBe(2)
	})

	test("getAllEmojiPacks caches until invalidated", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.applyState(imagePackEvent("pack1", { smile: { url: "mxc://example.com/smile" } }))
		const packs1 = room.getAllEmojiPacks()
		const packs2 = room.getAllEmojiPacks()
		expect(packs1).toBe(packs2)
		// Invalidating via image pack state change resets the cache
		room.applyState(imagePackEvent("pack1", { smile: { url: "mxc://example.com/smile2" } }))
		const packs3 = room.getAllEmojiPacks()
		expect(packs3).not.toBe(packs1)
	})

	test("image pack changes notify parent and imagePackSub", () => {
		const parent = makeParent()
		const room = new RoomStateStore(makeMeta(), parent)
		const sub = vi.fn()
		room.imagePackSub.subscribe(sub)
		room.applyState(imagePackEvent("pack1", { smile: { url: "mxc://example.com/smile" } }))
		expect(parent.invalidateEmojiPacksCache).toHaveBeenCalled()
		expect(sub).toHaveBeenCalled()
	})

	test("getAllBotCommands returns standard + fake + room commands", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.applyState(makeEvent({
			rowid: 40,
			event_id: "$cmd:example.com",
			type: "org.matrix.msc4391.command_description",
			state_key: "@bot:example.com",
			sender: "@bot:example.com",
			content: { command: "ping", description: "Ping the bot" },
		}))
		const commands = room.getAllBotCommands()
		// Standard commands are included
		expect(commands.some(cmd => cmd.source === fakeGomuksSender)).toBe(true)
		// Room command included
		expect(commands.some(cmd => cmd.command === "ping" && cmd.source === "@bot:example.com")).toBe(true)
	})

	test("getAllBotCommands filters non-joined owners when full member list loaded", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.applyState(makeEvent({
			rowid: 40,
			event_id: "$cmd:example.com",
			type: "org.matrix.msc4391.command_description",
			state_key: "@bot:example.com",
			sender: "@bot:example.com",
			content: { command: "ping" },
		}))
		// Mark full members loaded without a member event for the bot owner
		room.applyFullState([], false)
		const commands = room.getAllBotCommands()
		expect(commands.some(cmd => cmd.source === "@bot:example.com")).toBe(false)
	})

	test("getAllBotCommands skips redacted command events", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const evt = makeEvent({
			rowid: 40,
			event_id: "$cmd:example.com",
			type: "org.matrix.msc4391.command_description",
			state_key: "@bot:example.com",
			sender: "@bot:example.com",
			content: { command: "ping" },
		})
		evt.redacted_by = "$redaction:example.com"
		room.applyState(evt)
		const commands = room.getAllBotCommands()
		expect(commands.some(cmd => cmd.source === "@bot:example.com")).toBe(false)
	})

	test("getAllBotCommands caches results", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const c1 = room.getAllBotCommands()
		const c2 = room.getAllBotCommands()
		expect(c1).toBe(c2)
	})

	test("getViaServers picks the most common other server", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		for (const user of ["@a:big.com", "@b:big.com", "@c:small.com"]) {
			room.applyState(makeEvent({
				rowid: 50 + user.length,
				event_id: `$m-${user}:example.com`,
				type: "m.room.member",
				state_key: user,
				sender: user,
				content: { membership: "join" },
			}))
		}
		const vias = room.getViaServers()
		expect(vias[0]).toBe("example.com")
		// big.com has 2 members, small.com has 1 — big.com should be picked
		expect(vias).toContain("big.com")
		expect(vias).not.toContain("small.com")
	})

	test("getEmojiPack knock members are sorted last", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.applyState(makeEvent({
			rowid: 60,
			event_id: "$knock:example.com",
			type: "m.room.member",
			state_key: "@knocker:example.com",
			sender: "@knocker:example.com",
			content: { membership: "knock", displayname: "Zed" },
		}))
		room.applyState(makeEvent({
			rowid: 61,
			event_id: "$join:example.com",
			type: "m.room.member",
			state_key: "@joiner:example.com",
			sender: "@joiner:example.com",
			content: { membership: "join", displayname: "Amy" },
		}))
		const members = room.getMembers()
		expect(members.length).toBe(2)
		// Knocked member sorted last even though displayname would sort earlier
		expect(members[members.length - 1].userID).toBe("@knocker:example.com")
	})

	test("receipts: newer receipt replaces older and updates arrays", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.applyEvent(makeEvent({ rowid: 1, timeline_rowid: 10 }))
		room.applyEvent(makeEvent({ rowid: 2, timeline_rowid: 20, event_id: "$evt2:example.com" }))
		// First receipt on event 1
		room.applyReceipts(
			[{ user_id: "@a:example.com", receipt_type: "m.read", event_id: "$evt:example.com", timestamp: 100 }],
			"$evt:example.com", false,
		)
		expect(room.receiptsByUserID.get("@a:example.com")!.timeline_rowid).toBe(10)
		// Same user moves to event 2 (newer)
		room.applyReceipts(
			[{ user_id: "@a:example.com", receipt_type: "m.read", event_id: "$evt2:example.com", timestamp: 200 }],
			"$evt2:example.com", false,
		)
		expect(room.receiptsByUserID.get("@a:example.com")!.timeline_rowid).toBe(20)
		// Old receipt array for event 1 should no longer contain the user
		expect(room.receiptsByEventID.get("$evt:example.com") ?? []).not.toContainEqual(
			expect.objectContaining({ user_id: "@a:example.com" }),
		)
	})

	test("receipts: older receipt is ignored", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.applyEvent(makeEvent({ rowid: 1, timeline_rowid: 10 }))
		room.applyEvent(makeEvent({ rowid: 2, timeline_rowid: 20, event_id: "$evt2:example.com" }))
		room.applyReceipts(
			[{ user_id: "@a:example.com", receipt_type: "m.read", event_id: "$evt2:example.com", timestamp: 200 }],
			"$evt2:example.com", false,
		)
		// Older receipt (on event 1) should be rejected — an empty array entry may be set for evt1
		room.applyReceipts(
			[{ user_id: "@a:example.com", receipt_type: "m.read", event_id: "$evt:example.com", timestamp: 100 }],
			"$evt:example.com", false,
		)
		expect((room.receiptsByEventID.get("$evt:example.com") ?? []).length).toBe(0)
	})

	test("receipts: override replaces existing list", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.applyEvent(makeEvent({ rowid: 1, timeline_rowid: 10 }))
		room.applyReceipts(
			[{ user_id: "@a:example.com", receipt_type: "m.read", event_id: "$evt:example.com", timestamp: 100 }],
			"$evt:example.com", false,
		)
		room.applyReceipts(
			[{ user_id: "@b:example.com", receipt_type: "m.read", event_id: "$evt:example.com", timestamp: 50 }],
			"$evt:example.com", true,
		)
		const receipts = room.receiptsByEventID.get("$evt:example.com")!
		expect(receipts.length).toBe(1)
		expect(receipts[0].user_id).toBe("@b:example.com")
	})

	test("applyPagination applies related events and receipts", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const history = [makeEvent({ rowid: 1, timeline_rowid: 1 })]
		const related = [makeEvent({
			rowid: 2, timeline_rowid: 0, event_id: "$related:example.com", type: "m.reaction",
		})]
		room.applyPagination(history, related, {
			"$evt:example.com": [
				{ user_id: "@a:example.com", receipt_type: "m.read", event_id: "$evt:example.com", timestamp: 1 },
			],
		})
		expect(room.eventsByRowID.has(2)).toBe(true)
		expect(room.receiptsByEventID.get("$evt:example.com")).toBeDefined()
		expect(room.timeline.length).toBe(1)
	})

	test("applySendComplete ignores already-confirmed event", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.applyEvent(makeEvent({ rowid: 1 })) // non-pending
		const notifySpy = vi.spyOn(room, "notifyTimelineSubscribers")
		room.applySendComplete(makeEvent({ rowid: 1 }))
		expect(notifySpy).not.toHaveBeenCalled()
	})

	test("applySendComplete applies pending event", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.applySendComplete(makeEvent({ rowid: 5, timeline_rowid: 0, sender: USER }))
		expect(room.eventsByRowID.get(5)!.pending).toBe(true)
	})

	test("applySync receipts are applied", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.applyEvent(makeEvent({ rowid: 1 }))
		room.applySync({
			receipts: {
				"$evt:example.com": [
					{ user_id: "@a:example.com", receipt_type: "m.read", event_id: "$evt:example.com", timestamp: 1 },
				],
			},
		} as SyncRoom, false)
		expect(room.receiptsByEventID.get("$evt:example.com")).toBeDefined()
	})

	test("applySync notifies widget listeners of timeline and state events", () => {
		const parent = makeParent()
		const room = new RoomStateStore(makeMeta(), parent)
		const listener = {
			onTimelineEvent: vi.fn(),
			onStateEvent: vi.fn(),
			onToDeviceEvent: vi.fn(),
			onRoomChange: vi.fn(),
		}
		parent.widgetListeners.add(listener)
		const evt = makeEvent({ rowid: 1 })
		const stateEvt = makeEvent({
			rowid: 2, type: "m.room.name", state_key: "", content: { name: "X" },
		})
		room.applySync({
			events: [evt, stateEvt],
			timeline: [{ timeline_rowid: 1, event_rowid: 1 }],
			state: { "m.room.name": { "": 2 } },
			sticky: [1],
		} as SyncRoom, false)
		expect(listener.onTimelineEvent).toHaveBeenCalled()
		expect(listener.onStateEvent).toHaveBeenCalled()
	})

	test("applySync emits newTimelineEventSub for timeline events", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const emitted: unknown[] = []
		room.newTimelineEventSub.listen(evt => emitted.push(evt))
		const evt = makeEvent({ rowid: 1 })
		room.applySync({
			events: [evt],
			timeline: [{ timeline_rowid: 1, event_rowid: 1 }],
		} as SyncRoom, false)
		expect(emitted.length).toBe(1)
	})

	test("applySync with reset emits null to newTimelineEventSub", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		let emitted: unknown = "unset"
		room.newTimelineEventSub.listen(evt => {
			emitted = evt
		})
		room.applySync({ reset: true, timeline: [] } as SyncRoom, false)
		expect(emitted).toBeNull()
	})

	test("thread listener receives only thread events", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const appends: unknown[][] = []
		const overwrites: unknown[] = []
		room.subscribeThread("$thread:example.com", (append, overwrite) => {
			if (append) {
				appends.push(append)
			}
			if (overwrite) {
				overwrites.push(overwrite)
			}
		})
		const threadEvt = makeEvent({
			rowid: 1, content: { body: "in thread", "m.relates_to": { rel_type: "m.thread", event_id: "$thread:example.com" } },
		})
		const otherEvt = makeEvent({ rowid: 2, event_id: "$other:example.com", content: { body: "not in thread" } })
		room.applySync({
			events: [threadEvt, otherEvt],
			timeline: [{ timeline_rowid: 1, event_rowid: 1 }, { timeline_rowid: 2, event_rowid: 2 }],
		} as SyncRoom, false)
		// saveEventToMaps delivers the thread event as an overwrite during applyEvent,
		// applySync delivers it again in the filtered append batch
		expect(overwrites.length).toBe(1)
		expect(appends.length).toBe(1)
		expect((appends[0] as unknown[]).length).toBe(1)
	})

	test("getStateForCache includes preview member and last edit", () => {
		const room = new RoomStateStore(makeMeta({ preview_event_rowid: 1 }), makeParent())
		// Member event for the sender
		room.applyState(makeEvent({
			rowid: 2, type: "m.room.member", state_key: "@other:example.com", sender: "@other:example.com",
			content: { membership: "join" },
		}))
		// Edit event
		room.applyEvent(makeEvent({
			rowid: 3, event_id: "$edit:example.com", relation_type: "m.replace",
			relates_to: "$evt:example.com", content: { "m.new_content": { body: "edited" } },
		}))
		// Preview event with last_edit_rowid
		room.applyEvent(makeEvent({ rowid: 1, content: { body: "preview" }, last_edit_rowid: 3 }))
		const state = room.getStateForCache()
		expect(state.events.length).toBe(3) // preview + member + edit
		expect(state.state["m.room.member"]).toBeDefined()
	})

	test("doGarbageCollection keeps image pack state and preview edit", () => {
		const room = new RoomStateStore(makeMeta({ preview_event_rowid: 1 }), makeParent())
		room.applyState(imagePackEvent("", { smile: { url: "mxc://example.com/smile" } }))
		room.applyEvent(makeEvent({
			rowid: 3, event_id: "$edit:example.com", relation_type: "m.replace",
			relates_to: "$evt:example.com", content: { "m.new_content": { body: "edited" } },
		}))
		room.applyEvent(makeEvent({ rowid: 1, content: { body: "preview" }, last_edit_rowid: 3 }))
		room.applyState(makeEvent({
			rowid: 2, type: "m.room.member", state_key: "@other:example.com", sender: "@other:example.com",
			content: { membership: "join" },
		}))
		const [deletedEvents] = room.doGarbageCollection()
		expect(deletedEvents).toBe(0)
		// Emote state kept
		expect(room.state.get("m.room.image_pack")).toBeDefined()
		// All referenced events kept
		expect(room.eventsByRowID.has(1)).toBe(true)
		expect(room.eventsByRowID.has(2)).toBe(true)
		expect(room.eventsByRowID.has(3)).toBe(true)
		expect(room.stateLoaded).toBe(false)
		expect(room.fullMembersLoaded).toBe(false)
	})

	test("applyDecrypted notifies timeline subscribers when timeline event decrypted", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		room.applyEvent(makeEvent({ rowid: 1, type: "m.room.encrypted" }))
		room.timeline = [{ timeline_rowid: 1, event_rowid: 1 }]
		const notifySpy = vi.spyOn(room, "notifyTimelineSubscribers")
		room.applyDecrypted({
			room_id: "!room:example.com",
			events: [makeEvent({
				rowid: 1, type: "m.room.encrypted", decrypted: { body: "now" }, decrypted_type: "m.room.message",
			})],
		})
		expect(notifySpy).toHaveBeenCalled()
	})

	test("applyDecrypted with stateCache set persists room", () => {
		const stateCache = { setRoom: vi.fn() }
		const parent = makeParent({ stateCache })
		const room = new RoomStateStore(makeMeta(), parent)
		room.applyDecrypted({
			room_id: "!room:example.com",
			preview_event_rowid: 9,
			events: [],
		})
		expect(stateCache.setRoom).toHaveBeenCalled()
	})

	test("applySync persists room to stateCache when meta changed", () => {
		const stateCache = { setRoom: vi.fn() }
		const parent = makeParent({ stateCache })
		const room = new RoomStateStore(makeMeta(), parent)
		room.applySync({ meta: makeMeta({ name: "Changed" }) } as SyncRoom, false)
		expect(stateCache.setRoom).toHaveBeenCalled()
	})

	test("applySync room account data is stored and notifies stateCache", () => {
		const stateCache = { setRoomAccountData: vi.fn() }
		const parent = makeParent({ stateCache })
		const room = new RoomStateStore(makeMeta(), parent)
		room.applySync({
			account_data: {
				"m.fully_read": {
					user_id: USER, room_id: "!room:example.com", type: "m.fully_read", content: { event_id: "$x" },
				},
			},
		} as SyncRoom, false)
		expect(room.accountData.get("m.fully_read")).toBeDefined()
		expect(stateCache.setRoomAccountData).toHaveBeenCalled()
	})

	test("applySync gomuks room preferences update server cache", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const notifySpy = vi.fn()
		room.preferenceSub.subscribe(notifySpy)
		room.applySync({
			account_data: {
				"fi.mau.gomuks.preferences": {
					user_id: USER, room_id: "!room:example.com",
					type: "fi.mau.gomuks.preferences", content: { send_typing_notifications: false },
				},
			},
		} as SyncRoom, false)
		expect(room.serverPreferenceCache.send_typing_notifications).toBe(false)
		expect(notifySpy).toHaveBeenCalled()
	})

	test("removeFailedEvent ignores non-pending event", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const evt = makeEvent({ rowid: 5 })
		room.applyEvent(evt)
		// Not in pendingEvents → no-op
		expect(() => room.removeFailedEvent(room.eventsByRowID.get(5)!)).not.toThrow()
	})

	test("setViewingRedacted toggles back to false", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const evt = makeEvent({ rowid: 1, redacted_by: "$redaction:example.com" })
		room.applyEvent(evt)
		const memEvt = room.eventsByRowID.get(1)!
		room.setViewingRedacted(memEvt, true)
		expect(room.eventsByRowID.get(1)!.viewing_redacted).toBe(true)
		room.setViewingRedacted(room.eventsByRowID.get(1)!, false)
		expect(room.eventsByRowID.get(1)!.viewing_redacted).toBe(false)
	})

	test("getOrApplyEvent returns existing event", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const evt = makeEvent({ rowid: 1 })
		room.applyEvent(evt)
		const existing = room.getOrApplyEvent(makeEvent({ rowid: 1, content: { body: "different" } }))
		expect(existing).toBe(room.eventsByRowID.get(1))
	})

	test("getOrApplyEvent applies new event", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const evt = makeEvent({ rowid: 2, event_id: "$new:example.com" })
		const result = room.getOrApplyEvent(evt)
		expect(result.mem).toBe(true)
		expect(room.eventsByID.has("$new:example.com")).toBe(true)
	})

	test("applyEvent with viewRedacted sets viewing_redacted", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const evt = makeEvent({ rowid: 1, redacted_by: "$redaction:example.com" })
		room.applyEvent(evt, false, true)
		expect(room.eventsByRowID.get(1)!.viewing_redacted).toBe(true)
	})

	test("applyEvent marks own messages as edit targets", () => {
		const parent = makeParent()
		const room = new RoomStateStore(makeMeta(), parent)
		room.timeline = [{ timeline_rowid: 1, event_rowid: 1 }]
		room.applyEvent(makeEvent({ rowid: 1, sender: USER }))
		room.notifyTimelineSubscribers()
		expect(room.editTargets).toContain(1)
	})

	test("applyEvent ignores own edits as edit targets", () => {
		const parent = makeParent()
		const room = new RoomStateStore(makeMeta(), parent)
		room.timeline = [{ timeline_rowid: 1, event_rowid: 1 }]
		room.applyEvent(makeEvent({
			rowid: 1, sender: USER, relation_type: "m.replace", relates_to: "$target:example.com",
		}))
		room.notifyTimelineSubscribers()
		expect(room.editTargets).not.toContain(1)
	})

	test("unread notifications zero closes open notifications", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const notif = { close: vi.fn() }
		room.openNotifications.set(1, notif as unknown as Notification)
		room.applySync({
			meta: makeMeta({ unread_notifications: 0, unread_highlights: 0, sorting_timestamp: 2000 }),
		} as SyncRoom, false)
		expect(notif.close).toHaveBeenCalled()
	})

	test("applyFullState notifies imagePackSub for emote state types", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const sub = vi.fn()
		room.imagePackSub.subscribe(sub)
		room.applyFullState([imagePackEvent("pack1", { smile: { url: "mxc://x/y" } })], false)
		expect(sub).toHaveBeenCalled()
	})

	test("applyFullState throws when state key missing", () => {
		const room = new RoomStateStore(makeMeta(), makeParent())
		const evt = makeEvent({ type: "m.room.name" })
		delete evt.state_key
		expect(() => room.applyFullState([evt], false)).toThrow("missing state key")
	})
})
