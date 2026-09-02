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
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest"
import {
	DBInvitedRoom,
	DBRoom,
	RawDBEvent,
	RoomNameQuality,
	SyncCompleteData,
	SyncRoom,
	UnreadType,
} from "../types"
import { RoomListEntry, StateStore, WidgetListener } from "./main.ts"

vi.mock("@/util/sound.ts", () => ({
	playSound: vi.fn(),
}))

// The local preference cache persists to localStorage — reset between tests so
// preference changes in one test don't leak into other tests' sort behavior.
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

function makeSyncRoom(overrides: Partial<SyncRoom> = {}): SyncRoom {
	return {
		meta: makeMeta(),
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

function makeInvite(overrides: Partial<DBInvitedRoom> = {}): DBInvitedRoom {
	return {
		room_id: "!invite:example.com",
		created_at: 1700000000000,
		invite_state: [
			{ type: "m.room.name", state_key: "", sender: "@x:example.com", content: { name: "Invite" } },
			{ type: "m.room.member", state_key: USER, sender: "@x:example.com", content: { membership: "invite" } },
		],
		...overrides,
	}
}

function makeStore(): StateStore {
	const store = new StateStore()
	store.userID = USER
	return store
}

describe("StateStore basics", () => {
	test("initial state", () => {
		const store = makeStore()
		expect(store.rooms.size).toBe(0)
		expect(store.inviteRooms.size).toBe(0)
		expect(store.roomList.current).toEqual([])
		expect(store.activeRoomID).toBeNull()
		expect(store.roomListFilterFunc).toBeNull()
		expect(store.getFilteredRoomList()).toEqual([])
		expect(store.stateCacheStatus).toBe("disabled")
	})

	test("activeRoomID setter notifies widget listeners", () => {
		const store = makeStore()
		const listener: WidgetListener = {
			onTimelineEvent: vi.fn(),
			onStateEvent: vi.fn(),
			onToDeviceEvent: vi.fn(),
			onRoomChange: vi.fn(),
		}
		store.widgetListeners.add(listener)
		store.activeRoomID = "!room:example.com"
		expect(listener.onRoomChange).toHaveBeenCalledWith("!room:example.com")
	})

	test("getSpaceByID returns null for undefined", () => {
		const store = makeStore()
		expect(store.getSpaceByID(undefined)).toBeNull()
	})

	test("getSpaceByID returns pseudo spaces by id (homeSpace is NOT in pseudoSpaces)", () => {
		const store = makeStore()
		// homeSpace is not in pseudoSpaces (only orphans, directChats, unreads)
		expect(store.getSpaceByID("fi.mau.gomuks.home")).toBeNull()
		expect(store.getSpaceByID("fi.mau.gomuks.direct_chats")).toBe(store.directChatsSpace)
		expect(store.getSpaceByID("fi.mau.gomuks.unreads")).toBe(store.unreadsSpace)
		expect(store.getSpaceByID("fi.mau.gomuks.space_orphans")).toBe(store.spaceOrphans)
	})

	test("getSpaceByID returns null for unknown space", () => {
		const store = makeStore()
		expect(store.getSpaceByID("unknown")).toBeNull()
	})

	test("clear resets state", () => {
		const store = makeStore()
		store.applySync({
			rooms: { "!room:example.com": makeSyncRoom() },
		})
		expect(store.rooms.size).toBe(1)
		store.clear()
		expect(store.rooms.size).toBe(0)
		expect(store.roomList.current).toEqual([])
		expect(store.topLevelSpaces.current).toEqual([])
		expect(store.activeRoomID).toBeNull()
	})
})

describe("applySync", () => {
	test("creates room from sync", () => {
		const store = makeStore()
		store.applySync({
			rooms: { "!room:example.com": makeSyncRoom() },
		})
		expect(store.rooms.size).toBe(1)
		expect(store.rooms.get("!room:example.com")).toBeDefined()
		expect(store.roomList.current.length).toBe(1)
		expect(store.roomList.current[0].name).toBe("Test Room")
	})

	test("warns and skips unknown room without meta", () => {
		const store = makeStore()
		const warn = console.warn
		console.warn = vi.fn()
		store.applySync({ rooms: { "!unknown:example.com": {} as SyncRoom } })
		console.warn = warn
		expect(store.rooms.size).toBe(0)
	})

	test("hides rooms with unsupported creation type", () => {
		const store = makeStore()
		store.applySync({
			rooms: {
				"!space:example.com": makeSyncRoom({
					meta: makeMeta({
						room_id: "!space:example.com",
						creation_content: { type: "m.space" },
					}),
				}),
			},
		})
		expect(store.rooms.size).toBe(1)
		expect(store.roomList.current.length).toBe(0)
		expect(store.rooms.get("!space:example.com")!.hidden).toBe(true)
	})

	test("shows rooms with supported creation types", () => {
		const store = makeStore()
		store.applySync({
			rooms: {
				"!call:example.com": makeSyncRoom({
					meta: makeMeta({
						room_id: "!call:example.com",
						creation_content: { type: "org.matrix.msc3417.call" },
					}),
				}),
			},
		})
		expect(store.roomList.current.length).toBe(1)
	})

	test("hides tombstoned rooms with valid replacement", () => {
		const store = makeStore()
		// Create replacement room first
		store.applySync({
			rooms: {
				"!new:example.com": makeSyncRoom({
					meta: makeMeta({
						room_id: "!new:example.com",
						creation_content: { predecessor: { room_id: "!old:example.com" } },
					}),
				}),
			},
		})
		// Sync the old room which is tombstoned pointing at the new room
		store.applySync({
			rooms: {
				"!old:example.com": makeSyncRoom({
					meta: makeMeta({
						room_id: "!old:example.com",
						tombstone: { replacement_room: "!new:example.com" },
					}),
				}),
			},
		})
		const oldRoom = store.rooms.get("!old:example.com")
		expect(oldRoom?.tombstoned).toBe(true)
		expect(oldRoom?.hidden).toBe(true)
		// old room should not be in the room list anymore
		expect(store.roomList.current.find(r => r.room_id === "!old:example.com")).toBeUndefined()
	})

	test("joining replacement room hides tombstoned predecessor", () => {
		const store = makeStore()
		// Old room synced with tombstone
		store.applySync({
			rooms: {
				"!old:example.com": makeSyncRoom({
					meta: makeMeta({
						room_id: "!old:example.com",
						tombstone: { replacement_room: "!new:example.com" },
					}),
				}),
			},
		})
		expect(store.roomList.current.length).toBe(1)
		// New room joined with predecessor reference
		store.applySync({
			rooms: {
				"!new:example.com": makeSyncRoom({
					meta: makeMeta({
						room_id: "!new:example.com",
						creation_content: { predecessor: { room_id: "!old:example.com" } },
					}),
				}),
			},
		})
		expect(store.rooms.get("!old:example.com")!.hidden).toBe(true)
		expect(store.roomList.current.find(r => r.room_id === "!old:example.com")).toBeUndefined()
	})

	test("clear_state clears existing rooms and restores active room", () => {
		const store = makeStore()
		const switchRoom = vi.fn()
		store.switchRoom = switchRoom
		store.applySync({ rooms: { "!room:example.com": makeSyncRoom() } })
		store.activeRoomID = "!room:example.com"
		store.applySync({
			clear_state: true,
			rooms: { "!room:example.com": makeSyncRoom({ meta: makeMeta({ sorting_timestamp: 2000 }) }) },
		})
		expect(store.rooms.size).toBe(1)
		expect(switchRoom).toHaveBeenCalledWith("!room:example.com")
	})

	test("catchup triggers garbage collection on rooms", () => {
		const store = makeStore()
		store.applySync({ rooms: { "!room:example.com": makeSyncRoom() } })
		const room = store.rooms.get("!room:example.com")!
		const gcSpy = vi.spyOn(room, "doGarbageCollection")
		store.applySync({ catchup: true, rooms: {} })
		expect(gcSpy).toHaveBeenCalled()
	})

	test("updates existing room meta", () => {
		const store = makeStore()
		store.applySync({ rooms: { "!room:example.com": makeSyncRoom() } })
		store.applySync({
			rooms: {
				"!room:example.com": makeSyncRoom({
					meta: makeMeta({ sorting_timestamp: 2000, unread_messages: 3 }),
				}),
			},
		})
		expect(store.rooms.get("!room:example.com")!.meta.current.sorting_timestamp).toBe(2000)
		expect(store.roomList.current[0].unread_messages).toBe(3)
	})

	test("applies events and timeline", () => {
		const store = makeStore()
		const evt = makeEvent()
		store.applySync({
			rooms: {
				"!room:example.com": makeSyncRoom({
					events: [evt],
					timeline: [{ timeline_rowid: 1, event_rowid: 1 }],
				}),
			},
		})
		const room = store.rooms.get("!room:example.com")!
		expect(room.eventsByRowID.get(1)).toBeDefined()
		expect(room.timeline.length).toBe(1)
	})

	test("preview event appears in room list entry", () => {
		const store = makeStore()
		const evt = makeEvent({ rowid: 5, content: { body: "preview msg" } })
		store.applySync({
			rooms: {
				"!room:example.com": makeSyncRoom({
					meta: makeMeta({ preview_event_rowid: 5 }),
					events: [evt],
				}),
			},
		})
		expect(store.roomList.current[0].preview_event?.content.body).toBe("preview msg")
	})

	test("m.tag account data affects favorite/low priority", () => {
		const store = makeStore()
		store.applySync({ rooms: { "!room:example.com": makeSyncRoom() } })
		// Sync again with m.tag account data marking the room favourite + lowpriority
		store.applySync({
			rooms: {
				"!room:example.com": makeSyncRoom({
					meta: makeMeta({ sorting_timestamp: 3000 }),
					account_data: {
						"m.tag": {
							user_id: USER,
							room_id: "!room:example.com",
							type: "m.tag",
							content: { tags: { "m.favourite": { order: 0.5 }, "m.lowpriority": {} } },
						},
					},
				}),
			},
		})
		const entry = store.roomList.current[0]
		expect(entry.favorite_order).toBe(0.5)
		expect(entry.low_priority).toBe(true)
	})

	test("invite rooms appear in room list", () => {
		const store = makeStore()
		store.applySync({ invited_rooms: [makeInvite()] })
		expect(store.inviteRooms.size).toBe(1)
		expect(store.roomList.current.length).toBe(1)
		expect(store.roomList.current[0].is_invite).toBe(true)
	})

	test("joining invite removes it and adds room", () => {
		const store = makeStore()
		store.applySync({ invited_rooms: [makeInvite()] })
		expect(store.inviteRooms.size).toBe(1)
		store.applySync({
			rooms: { "!invite:example.com": makeSyncRoom({ meta: makeMeta({ room_id: "!invite:example.com" }) }) },
		})
		expect(store.inviteRooms.size).toBe(0)
		expect(store.rooms.size).toBe(1)
	})

	test("left rooms are removed", () => {
		const store = makeStore()
		store.applySync({ rooms: { "!room:example.com": makeSyncRoom() } })
		expect(store.rooms.size).toBe(1)
		store.applySync({ left_rooms: ["!room:example.com"] })
		expect(store.rooms.size).toBe(0)
		expect(store.roomList.current.length).toBe(0)
	})

	test("leaving active room switches away", () => {
		const store = makeStore()
		const switchRoom = vi.fn()
		store.switchRoom = switchRoom
		store.applySync({ rooms: { "!room:example.com": makeSyncRoom() } })
		store.activeRoomID = "!room:example.com"
		store.applySync({ left_rooms: ["!room:example.com"] })
		expect(switchRoom).toHaveBeenCalledWith(null)
	})

	test("account data is stored", () => {
		const store = makeStore()
		store.applySync({
			account_data: {
				"io.element.recent_emoji": {
					user_id: USER,
					type: "io.element.recent_emoji",
					content: { recent_emoji: [["😀", 5]] },
				},
			},
		})
		expect(store.accountData.get("io.element.recent_emoji")).toEqual({ recent_emoji: [["😀", 5]] })
	})

	test("gomuks preferences account data updates server cache", () => {
		const store = makeStore()
		const notifySpy = vi.fn()
		store.preferenceSub.subscribe(notifySpy)
		store.applySync({
			account_data: {
				"fi.mau.gomuks.preferences": {
					user_id: USER,
					type: "fi.mau.gomuks.preferences",
					content: { pin_favorites: true },
				},
			},
		})
		expect(store.serverPreferenceCache.pin_favorites).toBe(true)
		expect(notifySpy).toHaveBeenCalled()
	})

	test("space edges are applied", () => {
		const store = makeStore()
		store.applySync({
			rooms: {
				"!space:example.com": makeSyncRoom({
					meta: makeMeta({
						room_id: "!space:example.com",
						creation_content: { type: "m.space" },
					}),
				}),
			},
			space_edges: { "!space:example.com": [{ child_id: "!room:example.com" }] },
			top_level_spaces: ["!space:example.com"],
		})
		expect(store.topLevelSpaces.current).toEqual(["!space:example.com"])
		expect(store.spaceEdges.size).toBe(1)
		expect(store.getSpaceByID("!space:example.com")).toBeDefined()
	})

	test("alphabetical_order preference sorts room list (descending in array = A at top in UI)", () => {
		const store = makeStore()
		store.localPreferenceCache.alphabetical_order = true
		store.applySync({
			rooms: {
				"!a:example.com": makeSyncRoom({ meta: makeMeta({ room_id: "!a:example.com", name: "Alpha" }) }),
				"!z:example.com": makeSyncRoom({ meta: makeMeta({ room_id: "!z:example.com", name: "Zulu" }) }),
			},
		})
		// alphabeticalSort is r2.localeCompare(r1) → descending in array (Z first), displayed reversed (A at top)
		expect(store.roomList.current[0].name).toBe("Zulu")
		expect(store.roomList.current[1].name).toBe("Alpha")
	})

	test("pin_favorites keeps favorites at end of array (displayed at top in UI)", () => {
		const store = makeStore()
		store.localPreferenceCache.pin_favorites = true
		// Create two rooms, favorite the one with older timestamp
		store.applySync({
			rooms: {
				"!recent:example.com": makeSyncRoom({
					meta: makeMeta({ room_id: "!recent:example.com", name: "Recent", sorting_timestamp: 200 }),
				}),
				"!fav:example.com": makeSyncRoom({
					meta: makeMeta({
						room_id: "!fav:example.com",
						name: "Favorite",
						sorting_timestamp: 100,
					}),
					account_data: {
						"m.tag": {
							user_id: USER,
							room_id: "!fav:example.com",
							type: "m.tag",
							content: { tags: { "m.favourite": { order: 1 } } },
						},
					},
				}),
			},
		})
		// favoriteSort: rooms with favorite_order go AFTER rooms without (in array), displayed at top
		expect(store.roomList.current[0].name).toBe("Recent")
		expect(store.roomList.current[1].name).toBe("Favorite")
	})

	test("pin_low_priority keeps low priority rooms at start of array (displayed at bottom in UI)", () => {
		const store = makeStore()
		store.localPreferenceCache.pin_low_priority = true
		store.applySync({
			rooms: {
				"!normal:example.com": makeSyncRoom({
					meta: makeMeta({ room_id: "!normal:example.com", name: "Normal", sorting_timestamp: 100 }),
				}),
				"!low:example.com": makeSyncRoom({
					meta: makeMeta({ room_id: "!low:example.com", name: "Low", sorting_timestamp: 300 }),
					account_data: {
						"m.tag": {
							user_id: USER,
							room_id: "!low:example.com",
							type: "m.tag",
							content: { tags: { "m.lowpriority": {} } },
						},
					},
				}),
			},
		})
		// lowPrioritySort: low priority rooms go BEFORE normal rooms in array, displayed at bottom
		expect(store.roomList.current[0].name).toBe("Low")
		expect(store.roomList.current[1].name).toBe("Normal")
	})

	test("mute_low_priority zeroes unread count", () => {
		const store = makeStore()
		store.localPreferenceCache.mute_low_priority = true
		store.applySync({
			rooms: {
				"!low:example.com": makeSyncRoom({
					meta: makeMeta({ room_id: "!low:example.com", unread_messages: 10 }),
					account_data: {
						"m.tag": {
							user_id: USER,
							room_id: "!low:example.com",
							type: "m.tag",
							content: { tags: { "m.lowpriority": {} } },
						},
					},
				}),
			},
		})
		expect(store.roomList.current[0].unread_messages).toBe(0)
	})

	test("unread modification updates home space counts", () => {
		const store = makeStore()
		store.applySync({
			rooms: {
				"!room:example.com": makeSyncRoom({
					meta: makeMeta({ unread_messages: 5, unread_notifications: 2, unread_highlights: 1 }),
				}),
			},
		})
		expect(store.homeSpace.counts.current).toEqual({
			unread_messages: 5,
			unread_notifications: 2,
			unread_highlights: 1,
		})
	})

	test("dm rooms update direct chats space", () => {
		const store = makeStore()
		store.applySync({
			rooms: {
				"!dm:example.com": makeSyncRoom({
					meta: makeMeta({
						room_id: "!dm:example.com",
						dm_user_id: "@other:example.com",
						unread_messages: 3,
					}),
				}),
			},
		})
		expect(store.directChatsSpace.counts.current.unread_messages).toBe(3)
	})

	test("unread rooms update home space counts (unreadsSpace.counts is never updated by applySync)", () => {
		const store = makeStore()
		store.applySync({
			rooms: {
				"!room:example.com": makeSyncRoom({
					meta: makeMeta({ unread_messages: 7 }),
				}),
			},
		})
		// unreadsSpace.counts is never updated by #applyUnreadModification (only homeSpace/directChats/spaceOrphans/edge spaces)
		expect(store.unreadsSpace.counts.current).toEqual({
			unread_messages: 0,
			unread_notifications: 0,
			unread_highlights: 0,
		})
		// But include() works dynamically
		expect(store.unreadsSpace.include(store.roomList.current[0])).toBe(true)
	})

	test("new room entries are inserted in sorted position (ascending timestamps)", () => {
		const store = makeStore()
		// Initial sync with one room
		store.applySync({
			rooms: { "!a:example.com": makeSyncRoom({ meta: makeMeta({ room_id: "!a:example.com", sorting_timestamp: 100 }) }) },
		})
		// Second sync adds a room with newer timestamp
		store.applySync({
			rooms: {
				"!b:example.com": makeSyncRoom({
					meta: makeMeta({ room_id: "!b:example.com", sorting_timestamp: 200, unread_messages: 1 }),
				}),
			},
		})
		// Third sync adds a room newer than everything
		store.applySync({
			rooms: {
				"!c:example.com": makeSyncRoom({
					meta: makeMeta({ room_id: "!c:example.com", sorting_timestamp: 300, unread_messages: 1 }),
				}),
			},
		})
		// Array is ascending (oldest first, newest last); UI renders reversed
		expect(store.roomList.current.map(r => r.room_id)).toEqual(
			["!a:example.com", "!b:example.com", "!c:example.com"],
		)
	})

	test("new room entry with oldest timestamp is unshifted to start", () => {
		const store = makeStore()
		store.applySync({
			rooms: { "!b:example.com": makeSyncRoom({ meta: makeMeta({ room_id: "!b:example.com", sorting_timestamp: 200 }) }) },
		})
		store.applySync({
			rooms: {
				"!a:example.com": makeSyncRoom({
					meta: makeMeta({ room_id: "!a:example.com", sorting_timestamp: 50, unread_messages: 1 }),
				}),
			},
		})
		expect(store.roomList.current.map(r => r.room_id)).toEqual(["!a:example.com", "!b:example.com"])
	})

	test("new room entry with middle timestamp is spliced in correct position", () => {
		const store = makeStore()
		store.applySync({
			rooms: {
				"!a:example.com": makeSyncRoom({ meta: makeMeta({ room_id: "!a:example.com", sorting_timestamp: 100 }) }),
				"!c:example.com": makeSyncRoom({ meta: makeMeta({ room_id: "!c:example.com", sorting_timestamp: 300 }) }),
			},
		})
		store.applySync({
			rooms: {
				"!b:example.com": makeSyncRoom({
					meta: makeMeta({ room_id: "!b:example.com", sorting_timestamp: 200, unread_messages: 1 }),
				}),
			},
		})
		expect(store.roomList.current.map(r => r.room_id)).toEqual(
			["!a:example.com", "!b:example.com", "!c:example.com"],
		)
	})

	test("switches to room when active room is preview and synced", () => {
		const store = makeStore()
		const switchRoom = vi.fn()
		store.switchRoom = switchRoom
		store.activeRoomIsPreview = true
		store.applySync({
			rooms: { "!room:example.com": makeSyncRoom() },
			invited_rooms: [],
		})
		store.activeRoomID = "!room:example.com"
		store.applySync({
			rooms: {
				"!room:example.com": makeSyncRoom({
					meta: makeMeta({ sorting_timestamp: 5000, unread_messages: 1 }),
				}),
			},
		})
		expect(switchRoom).toHaveBeenCalledWith("!room:example.com")
	})

	test("switches to room when invite room for active room arrives", () => {
		const store = makeStore()
		const switchRoom = vi.fn()
		store.switchRoom = switchRoom
		store.activeRoomID = "!invite:example.com"
		store.applySync({
			invited_rooms: [makeInvite()],
			rooms: {},
		})
		expect(switchRoom).toHaveBeenCalledWith("!invite:example.com")
	})

	test("widget listeners receive to_device events", () => {
		const store = makeStore()
		const listener: WidgetListener = {
			onTimelineEvent: vi.fn(),
			onStateEvent: vi.fn(),
			onToDeviceEvent: vi.fn(),
			onRoomChange: vi.fn(),
		}
		store.widgetListeners.add(listener)
		store.applySync({
			to_device: [{ sender: "@x:example.com", type: "m.room.message", content: {}, encrypted: false }],
		})
		expect(listener.onToDeviceEvent).toHaveBeenCalled()
	})

	test("server timestamp is recorded", () => {
		const store = makeStore()
		store.applySync({ server_timestamp: 12345 })
		expect(store.serverTimestamp).toBe(12345)
	})

	test("notifications are shown when permission granted and unfocused", async () => {
		const store = makeStore()
		const showNotif = vi.fn()
		;(store as unknown as { showNotification: typeof showNotif }).showNotification = showNotif
		// Mock Notification.permission and focused state
		const { focused } = await import("@/util/focus.ts")
		focused.emit(false)
		const origNotification = window.Notification
		window.Notification = class MockNotification { static permission = "granted" } as unknown as typeof Notification
		try {
			store.applySync({
				rooms: {
					"!room:example.com": makeSyncRoom({
						notifications: [{ event_rowid: 1, sound: true }],
					}),
				},
			})
		} finally {
			window.Notification = origNotification
		}
		expect(showNotif).toHaveBeenCalled()
	})

	test("local web_push preference suppresses notifications", async () => {
		const store = makeStore()
		const showNotif = vi.fn()
		;(store as unknown as { showNotification: typeof showNotif }).showNotification = showNotif
		store.localPreferenceCache.web_push = true
		const { focused } = await import("@/util/focus.ts")
		focused.emit(false)
		const origNotification = window.Notification
		window.Notification = class MockNotification { static permission = "granted" } as unknown as typeof Notification
		try {
			store.applySync({
				rooms: {
					"!room:example.com": makeSyncRoom({
						notifications: [{ event_rowid: 1, sound: true }],
					}),
				},
			})
		} finally {
			window.Notification = origNotification
		}
		expect(showNotif).not.toHaveBeenCalled()
	})
})

describe("room list filtering", () => {
	test("query filters room list", () => {
		const store = makeStore()
		store.applySync({
			rooms: {
				"!a:example.com": makeSyncRoom({ meta: makeMeta({ room_id: "!a:example.com", name: "Alpha" }) }),
				"!b:example.com": makeSyncRoom({ meta: makeMeta({ room_id: "!b:example.com", name: "Beta" }) }),
			},
		})
		store.currentRoomListQuery = "alp"
		const filtered = store.getFilteredRoomList()
		expect(filtered.length).toBe(1)
		expect(filtered[0].name).toBe("Alpha")
	})

	test("custom filter function", () => {
		const store = makeStore()
		store.applySync({
			rooms: {
				"!a:example.com": makeSyncRoom({ meta: makeMeta({ room_id: "!a:example.com", unread_messages: 5 }) }),
				"!b:example.com": makeSyncRoom({ meta: makeMeta({ room_id: "!b:example.com" }) }),
			},
		})
		store.currentRoomListFilter = store.unreadsSpace
		const filtered = store.getFilteredRoomList()
		expect(filtered.length).toBe(1)
		expect(filtered[0].room_id).toBe("!a:example.com")
	})

	test("roomListFilterFunc is null when no query/filter", () => {
		const store = makeStore()
		expect(store.roomListFilterFunc).toBeNull()
	})

	test("findMatchingSpace returns orphan space for regular rooms", () => {
		const store = makeStore()
		const entry: RoomListEntry = {
			room_id: "!room:example.com",
			sorting_timestamp: 1000,
			name: "Test",
			search_name: "test",
			unread_messages: 0,
			unread_notifications: 0,
			unread_highlights: 0,
			marked_unread: false,
		}
		expect(store.findMatchingSpace(entry)).toBe(store.spaceOrphans)
	})

	test("findMatchingSpace returns direct chats space for DMs", () => {
		const store = makeStore()
		const entry: RoomListEntry = {
			room_id: "!dm:example.com",
			dm_user_id: "@x:example.com",
			sorting_timestamp: 1000,
			name: "DM",
			search_name: "dm",
			unread_messages: 0,
			unread_notifications: 0,
			unread_highlights: 0,
			marked_unread: false,
		}
		expect(store.findMatchingSpace(entry)).toBe(store.directChatsSpace)
	})
})

describe("emoji", () => {
	test("getEmojiPackKeys with no account data returns empty", () => {
		const store = makeStore()
		expect(store.getEmojiPackKeys()).toEqual([])
		expect(store.getEmojiPackKeys(false)).toEqual([])
	})

	test("getEmojiPackKeys generates keys from emote rooms", () => {
		const store = makeStore()
		store.accountData.set("im.ponies.emote_rooms", {
			rooms: { "!room:example.com": { "": {} } },
		})
		const keys = store.getEmojiPackKeys()
		expect(keys).toContainEqual({
			room_id: "!room:example.com",
			type: "im.ponies.room_emotes",
			state_key: "",
		})
		expect(keys).toContainEqual({
			room_id: "!room:example.com",
			type: "m.room.image_pack",
			state_key: "",
		})
	})

	test("invalidateEmojiPackKeyCache resets caches", () => {
		const store = makeStore()
		store.accountData.set("im.ponies.emote_rooms", { rooms: { "!r:e": { "": {} } } })
		store.getEmojiPackKeys()
		store.invalidateEmojiPackKeyCache()
		// After invalidation, keys should be regenerated
		const keys = store.getEmojiPackKeys()
		expect(keys.length).toBe(2)
	})

	test("frequentlyUsedEmoji parses recent emoji", () => {
		const store = makeStore()
		store.accountData.set("io.element.recent_emoji", {
			recent_emoji: [["🎉", 1], ["😀", 5]],
		})
		const freq = store.frequentlyUsedEmoji
		expect(freq.get("😀")).toBe(5)
		expect(freq.get("🎉")).toBe(1)
	})

	test("frequentlyUsedEmoji with no data returns empty map", () => {
		const store = makeStore()
		expect(store.frequentlyUsedEmoji.size).toBe(0)
	})

	test("frequentlyUsedEmoji handles malformed data", () => {
		const store = makeStore()
		store.accountData.set("io.element.recent_emoji", {
			recent_emoji: 42, // number, not array — toSorted fails
		})
		expect(store.frequentlyUsedEmoji.size).toBe(0)
	})

	test("recent emoji account data invalidates frequently used cache", () => {
		const store = makeStore()
		store.accountData.set("io.element.recent_emoji", { recent_emoji: [["😀", 1]] })
		expect(store.frequentlyUsedEmoji.size).toBe(1)
		store.applySync({
			account_data: {
				"io.element.recent_emoji": {
					user_id: USER,
					type: "io.element.recent_emoji",
					content: { recent_emoji: [["🎉", 2], ["😀", 1]] },
				},
			},
		})
		expect(store.frequentlyUsedEmoji.size).toBe(2)
	})
})

describe("space stores", () => {
	test("getSpaceStore creates space edge store when forced", () => {
		const store = makeStore()
		const space = store.getSpaceStore("!space:example.com", true)
		expect(space).toBeDefined()
		expect(space.id).toBe("!space:example.com")
	})

	test("getSpaceStore without force returns null for non-space", () => {
		const store = makeStore()
		store.applySync({
			rooms: { "!room:example.com": makeSyncRoom() },
		})
		expect(store.getSpaceStore("!room:example.com")).toBeNull()
	})

	test("getSpaceStore without force returns existing store", () => {
		const store = makeStore()
		const created = store.getSpaceStore("!space:example.com", true)
		expect(store.getSpaceStore("!space:example.com")).toBe(created)
	})
})

describe("showNotification", () => {
	let notifications: unknown[]
	let origNotification: (typeof window)["Notification"]

	beforeEach(() => {
		notifications = []
		origNotification = window.Notification
		window.Notification = class MockNotification {
			static permission = "granted"
			constructor(public title: string, public options?: unknown) {
				notifications.push(this)
			}
			close = vi.fn()
			onclose: (() => void) | null = null
			onclick: (() => void) | null = null
		} as unknown as (typeof window)["Notification"]
	})

	afterEach(() => {
		window.Notification = origNotification
	})

	test("shows notification for event with body", () => {
		const store = makeStore()
		const evt = makeEvent({ rowid: 5, content: { body: "notif body" } })
		store.applySync({
			rooms: {
				"!room:example.com": makeSyncRoom({
					events: [evt],
				}),
			},
		})
		const room = store.rooms.get("!room:example.com")!
		store.showNotification(room, 5, false)
		expect(notifications.length).toBe(1)
		expect(room.openNotifications.size).toBe(1)
	})

	test("skips notification when event not found", () => {
		const store = makeStore()
		store.applySync({ rooms: { "!room:example.com": makeSyncRoom() } })
		const room = store.rooms.get("!room:example.com")!
		store.showNotification(room, 999, false)
		expect(notifications.length).toBe(0)
	})

	test("skips notification when body is not a string", () => {
		const store = makeStore()
		const evt = makeEvent({ rowid: 6, content: {} })
		store.applySync({
			rooms: { "!room:example.com": makeSyncRoom({ events: [evt] }) },
		})
		const room = store.rooms.get("!room:example.com")!
		store.showNotification(room, 6, false)
		expect(notifications.length).toBe(0)
	})

	test("truncates long bodies", () => {
		const store = makeStore()
		const longBody = "x".repeat(500)
		const evt = makeEvent({ rowid: 7, content: { body: longBody } })
		store.applySync({
			rooms: { "!room:example.com": makeSyncRoom({ events: [evt] }) },
		})
		const room = store.rooms.get("!room:example.com")!
		store.showNotification(room, 7, false)
		const notif = notifications[0] as { options: { body: string } }
		expect(notif.options.body.length).toBeLessThan(500)
		expect(notif.options.body).toContain("…")
	})

	test("does not show notification when desktop handles them", () => {
		const store = makeStore()
		;(window as unknown as Record<string, unknown>).gomuksDesktop = {
			getDisableNotifications: () => true,
		}
		const evt = makeEvent({ rowid: 8, content: { body: "x" } })
		store.applySync({
			rooms: { "!room:example.com": makeSyncRoom({ events: [evt] }) },
		})
		const room = store.rooms.get("!room:example.com")!
		store.showNotification(room, 8, false)
		expect(notifications.length).toBe(0)
		delete (window as unknown as Record<string, unknown>).gomuksDesktop
	})

	test("onClickNotification switches to room", () => {
		const store = makeStore()
		const switchRoom = vi.fn()
		store.switchRoom = switchRoom
		store.onClickNotification("!room:example.com")
		expect(switchRoom).toHaveBeenCalledWith("!room:example.com")
	})
})

describe("applySendComplete / applyDecrypted / applyTyping", () => {
	test("applySendComplete applies event to room", () => {
		const store = makeStore()
		store.applySync({ rooms: { "!room:example.com": makeSyncRoom() } })
		const room = store.rooms.get("!room:example.com")!
		const applySpy = vi.spyOn(room, "applySendComplete")
		store.applySendComplete({ event: makeEvent({ rowid: 9 }), error: null })
		expect(applySpy).toHaveBeenCalled()
	})

	test("applySendComplete ignores unknown room", () => {
		const store = makeStore()
		expect(() => store.applySendComplete({ event: makeEvent(), error: null })).not.toThrow()
	})

	test("applyDecrypted applies to room and updates preview", () => {
		const store = makeStore()
		const evt = makeEvent({ rowid: 10, content: { body: "decrypted" } })
		store.applySync({
			rooms: {
				"!room:example.com": makeSyncRoom({
					meta: makeMeta({ preview_event_rowid: 10 }),
					events: [evt],
				}),
			},
		})
		store.applyDecrypted({
			room_id: "!room:example.com",
			preview_event_rowid: 10,
			events: [makeEvent({
				rowid: 10,
				type: "m.room.encrypted",
				decrypted: { body: "now visible" },
				decrypted_type: "m.room.message",
			})],
		})
		const room = store.rooms.get("!room:example.com")!
		expect(room.eventsByRowID.get(10)!.content.body).toBe("now visible")
	})

	test("applyDecrypted ignores unknown room", () => {
		const store = makeStore()
		expect(() => store.applyDecrypted({ room_id: "!nope:example.com", events: [] })).not.toThrow()
	})

	test("applyTyping updates typing users", () => {
		const store = makeStore()
		store.applySync({ rooms: { "!room:example.com": makeSyncRoom() } })
		store.applyTyping({ room_id: "!room:example.com", user_ids: ["@a:example.com"] })
		const room = store.rooms.get("!room:example.com")!
		expect(room.typing).toEqual(["@a:example.com"])
	})

	test("applyTyping ignores unknown room", () => {
		const store = makeStore()
		expect(() => store.applyTyping({ room_id: "!nope:example.com", user_ids: [] })).not.toThrow()
	})

	test("clearTyping clears typing in all rooms", () => {
		const store = makeStore()
		store.applySync({ rooms: { "!room:example.com": makeSyncRoom() } })
		store.applyTyping({ room_id: "!room:example.com", user_ids: ["@a:example.com"] })
		store.clearTyping()
		const room = store.rooms.get("!room:example.com")!
		expect(room.typing).toEqual([])
	})
})

describe("doGarbageCollection", () => {
	test("skips active room and recently opened rooms", () => {
		const store = makeStore()
		store.applySync({ rooms: { "!room:example.com": makeSyncRoom() } })
		store.activeRoomID = "!room:example.com"
		const room = store.rooms.get("!room:example.com")!
		const gcSpy = vi.spyOn(room, "doGarbageCollection")
		store.doGarbageCollection()
		expect(gcSpy).not.toHaveBeenCalled()
	})

	test("collects rooms not recently opened", () => {
		const store = makeStore()
		store.applySync({ rooms: { "!room:example.com": makeSyncRoom() } })
		const room = store.rooms.get("!room:example.com")!
		room.lastOpened = 0
		const result = store.doGarbageCollection()
		expect(result).toEqual({ deletedEvents: 0, deletedState: 0 })
	})

	test("skips rooms with active widgets", () => {
		const store = makeStore()
		store.applySync({ rooms: { "!room:example.com": makeSyncRoom() } })
		const room = store.rooms.get("!room:example.com")!
		room.lastOpened = 0
		room.activeWidgets.add("widget1")
		const gcSpy = vi.spyOn(room, "doGarbageCollection")
		store.doGarbageCollection()
		expect(gcSpy).not.toHaveBeenCalled()
	})
})

describe("cache lifecycle", () => {
	test("closeCache with no cache just sets status", () => {
		const store = makeStore()
		store.closeCache()
		expect(store.stateCacheStatus).toBe("closed")
		expect(store.stateCache).toBeUndefined()
		expect(store.anyStateCache).toBeUndefined()
	})

	test("anyStateCache returns stateCache when set", () => {
		const store = makeStore()
		const fakeCache = { close: vi.fn() } as unknown as import("./cache.ts").default
		store.stateCache = fakeCache
		expect(store.anyStateCache).toBe(fakeCache)
		store.closeCache()
		expect(fakeCache.close).toHaveBeenCalled()
		expect(store.stateCacheStatus).toBe("closed")
	})

	test("getEmojiPackKeys with m.image_pack.rooms key", () => {
		const store = makeStore()
		store.accountData.set("m.image_pack.rooms", {
			rooms: { "!room:example.com": { packA: {} } },
		})
		const keys = store.getEmojiPackKeys()
		expect(keys).toContainEqual({
			room_id: "!room:example.com",
			type: "m.room.image_pack",
			state_key: "packA",
		})
	})

	test("getEmojiPackKeys handles malformed emote data (catch path)", () => {
		const store = makeStore()
		// packs is null → Object.keys(null) throws TypeError
		store.accountData.set("im.ponies.emote_rooms", { rooms: { room1: null } })
		const keys = store.getEmojiPackKeys()
		expect(keys).toEqual([])
	})

	test("getRoomEmojiPacks with missing room warns", () => {
		const store = makeStore()
		store.accountData.set("im.ponies.emote_rooms", {
			rooms: { "!missing:example.com": { pack: {} } },
		})
		const packs = store.getRoomEmojiPacks()
		expect(Object.keys(packs).length).toBe(0)
	})

	test("getRoomEmojiPacks with missing pack warns", () => {
		const store = makeStore()
		// Room exists but has no image pack state
		store.applySync({
			rooms: { "!room:example.com": makeSyncRoom() },
		})
		store.accountData.set("im.ponies.emote_rooms", {
			rooms: { "!room:example.com": { nonexistent: {} } },
		})
		const packs = store.getRoomEmojiPacks()
		expect(Object.keys(packs).length).toBe(0)
	})

	test("frequentlyUsedEmoji catch path with non-array data", () => {
		const store = makeStore()
		// recent_emoji is a number — toSorted doesn't exist → TypeError
		store.accountData.set("io.element.recent_emoji", { recent_emoji: 42 })
		const freq = store.frequentlyUsedEmoji
		expect(freq.size).toBe(0)
	})

	test("showNotification with senderName === roomName shows sender only", () => {
		const store = makeStore()
		store.applySync({
			rooms: {
				"!room:example.com": makeSyncRoom({
					meta: makeMeta({ name: "Alice" }),
				}),
			},
		})
		const room = store.rooms.get("!room:example.com")!
		// Set member event with displayname matching room name
		room.applyState(makeEvent({
			rowid: 10, type: "m.room.member", state_key: "@alice:example.com",
			sender: "@alice:example.com",
			content: { membership: "join", displayname: "Alice" },
		}))
		const evt = makeEvent({ rowid: 11, content: { body: "msg" }, sender: "@alice:example.com" })
		room.applyEvent(evt)
		// Capture Notification constructor args
		const notifications: { title: string }[] = []
		const origNotification = window.Notification
		window.Notification = class MockNotification {
			static permission = "granted"
			constructor(public title: string, public options?: unknown) {
				notifications.push({ title })
			}
			close = vi.fn()
			onclose: (() => void) | null = null
			onclick: (() => void) | null = null
		} as unknown as typeof Notification
		try {
			store.showNotification(room, 11, false)
		} finally {
			window.Notification = origNotification
		}
		// When senderName === roomName, title = senderName (no room name appended)
		expect(notifications[0].title).toBe("Alice")
	})

	test("showNotification with member event missing shows senderID", () => {
		const store = makeStore()
		store.applySync({
			rooms: { "!room:example.com": makeSyncRoom() },
		})
		const room = store.rooms.get("!room:example.com")!
		const evt = makeEvent({ rowid: 12, content: { body: "msg" }, sender: "@nobody:example.com" })
		room.applyEvent(evt)
		const notifications: { title: string }[] = []
		const origNotification = window.Notification
		window.Notification = class MockNotification {
			static permission = "granted"
			constructor(public title: string) { notifications.push({ title }) }
			close = vi.fn()
			onclose: (() => void) | null = null
			onclick: (() => void) | null = null
		} as unknown as typeof Notification
		try {
			store.showNotification(room, 12, false)
		} finally {
			window.Notification = origNotification
		}
		// getDisplayname strips @ and server → "nobody"
		expect(notifications[0].title).toContain("nobody")
	})

	test("applyDecrypted with preview_event_rowid but room not in roomList", () => {
		const store = makeStore()
		// Apply a room but don't create a room list entry (e.g. tombstoned)
		store.applySync({
			rooms: {
				"!room:example.com": makeSyncRoom({
					meta: makeMeta({
						creation_content: { type: "m.space" },
					}),
				}),
			},
		})
		// Room is hidden, not in roomList
		expect(store.roomList.current.length).toBe(0)
		// applyDecrypted with preview_event_rowid → findIndex returns -1 → no roomList update
		store.applyDecrypted({
			room_id: "!room:example.com",
			preview_event_rowid: 999,
			events: [],
		})
		expect(store.roomList.current.length).toBe(0)
	})

	test("applySync with invited_rooms when roomList already populated", () => {
		const store = makeStore()
		// First sync populates room list
		store.applySync({
			rooms: { "!room:example.com": makeSyncRoom() },
		})
		expect(store.roomList.current.length).toBe(1)
		// Second sync with invite — resyncRoomList is false
		store.applySync({
			invited_rooms: [makeInvite({ room_id: "!inv:example.com" })],
		})
		expect(store.inviteRooms.size).toBe(1)
		expect(store.roomList.current.length).toBe(2)
	})

	test("loadCache handles missing indexedDB gracefully", async () => {
		const store = makeStore()
		await store.loadCache()
		expect(store.stateCacheStatus).toMatch(/^failed:/)
	})

	test("deleteCache closes and marks deleted", async () => {
		const store = makeStore()
		await expect(store.deleteCache()).rejects.toBeDefined()
		expect(store.stateCacheStatus).toBe("deleted")
	})

	test("anyStateCache returns tmpStateCache as fallback", () => {
		const store = makeStore()
		const fakeCache = { close: vi.fn() } as unknown as import("./cache.ts").default
		store.tmpStateCache = fakeCache
		expect(store.anyStateCache).toBe(fakeCache)
	})
})
