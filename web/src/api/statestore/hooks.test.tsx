// gomuks - A Matrix client written in Go.
// Copyright (C) 2024 Tulir Asokan.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
import { act } from "react"
import { createRoot, type Root } from "react-dom/client"
import { createElement, useMemo } from "react"
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest"

vi.mock("@/api/client.ts", () => ({
	default: class MockClient {
		capabilities = { current: null }
		fetchCapabilities = vi.fn()
		requestMemberEvent = vi.fn().mockReturnValue(null)
	},
}))

import Client from "@/api/client.ts"
import {
	applyPerMessageSender,
	maybeRedactMemberEvent,
	useAccountData,
	useCustomEmojis,
	usePreferences,
	usePreference,
	useReadReceipts,
	useRoomAccountData,
	useRoomEvent,
	useRoomImagePacks,
	useRoomMember,
	useRoomMembers,
	useRoomState,
	useRoomTimeline,
	useRoomTyping,
	useSpaceEdges,
	useSubscribedPacks,
	useBotCommands,
	useCapabilities,
	useMultipleRoomMembers,
} from "./hooks.ts"
import { RoomStateStore } from "./room.ts"
import { StateStore } from "./main.ts"
import { NonNullCachedEventDispatcher } from "@/util/eventdispatcher.ts"
import type { DBRoom, RawDBEvent } from "../types"
import { RoomNameQuality, UnreadType } from "../types"

beforeEach(() => {
	(globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true
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

function makeStoreAndRoom() {
	const store = new StateStore()
	store.userID = USER
	const room = new RoomStateStore(makeMeta(), store)
	store.rooms.set(room.roomID, room)
	return { store, room }
}

describe("maybeRedactMemberEvent", () => {
	test("passes through normal member content", () => {
		const evt = { content: { membership: "join", displayname: "X" } } as never
		expect(maybeRedactMemberEvent(evt)).toEqual({ membership: "join", displayname: "X" })
	})

	test("null member event returns undefined", () => {
		expect(maybeRedactMemberEvent(null)).toBeUndefined()
	})

	test("redacted member event keeps only membership", () => {
		const evt = {
			content: { membership: "join", displayname: "X", avatar_url: "mxc://x" },
			redacted_by: "$redaction:example.com",
			viewing_redacted: false,
		} as never
		expect(maybeRedactMemberEvent(evt)).toEqual({ membership: "join" })
	})

	test("leave event without displayname falls back to prev_content", () => {
		const evt = {
			content: { membership: "leave" },
			unsigned: { prev_content: { membership: "join", displayname: "Old Name" } },
		} as never
		expect(maybeRedactMemberEvent(evt)).toEqual({ membership: "join", displayname: "Old Name" })
	})
})

describe("applyPerMessageSender", () => {
	test("returns member content when no per-message sender", () => {
		const content = { membership: "join", displayname: "Base" } as never
		expect(applyPerMessageSender(content, undefined)).toBe(content)
	})

	test("overrides with per-message sender profile", () => {
		const content = { membership: "join", displayname: "Base" } as never
		const result = applyPerMessageSender(content, {
			displayname: "Nick", avatar_url: "mxc://x/y",
		} as never)
		expect(result).toEqual({
			membership: "join",
			displayname: "Nick",
			avatar_url: "mxc://x/y",
			avatar_file: undefined,
		})
	})

	test("falls back to member fields when per-message sender incomplete", () => {
		const content = { membership: "join", displayname: "Base", avatar_url: "mxc://base" } as never
		const result = applyPerMessageSender(content, { displayname: "Nick" } as never)
		expect(result?.displayname).toBe("Nick")
		expect(result?.avatar_url).toBe("mxc://base")
	})
})

describe("hooks", () => {
	let container: HTMLDivElement
	let root: Root

	beforeEach(() => {
		container = document.createElement("div")
		document.body.appendChild(container)
		root = createRoot(container)
	})

	afterEach(() => {
		act(() => root.unmount())
		container.remove()
	})

	test("useRoomTimeline returns timelineCache", () => {
		const { room } = makeStoreAndRoom()
		room.applyEvent(makeEvent())
		room.timeline = [{ timeline_rowid: 1, event_rowid: 1 }]
		room.notifyTimelineSubscribers()
		let result: unknown
		function Test() {
			result = useRoomTimeline(room)
			return null
		}
		act(() => root.render(createElement(Test)))
		expect((result as unknown[]).length).toBe(1)
	})

	test("useRoomTimeline with undefined room returns empty array", () => {
		let result: unknown
		function Test() {
			result = useRoomTimeline(undefined)
			return null
		}
		act(() => root.render(createElement(Test)))
		expect(result).toEqual([])
	})

	test("useRoomTyping returns typing list", () => {
		const { room } = makeStoreAndRoom()
		room.applyTyping(["@a:example.com"])
		let result: unknown
		function Test() {
			result = useRoomTyping(room)
			return null
		}
		act(() => root.render(createElement(Test)))
		expect(result).toEqual(["@a:example.com"])
	})

	test("useCapabilities fetches and returns capabilities", () => {
		const client = new Client()
		client.capabilities = new NonNullCachedEventDispatcher({ client_versions: [] })
		let result: unknown
		function Test() {
			result = useCapabilities(client)
			return null
		}
		act(() => root.render(createElement(Test)))
		expect(client.fetchCapabilities).toHaveBeenCalled()
		expect(result).toEqual({ client_versions: [] })
	})

	test("useRoomState returns state event", () => {
		const { room } = makeStoreAndRoom()
		room.applyState(makeEvent({
			rowid: 1, type: "m.room.name", state_key: "", content: { name: "Named" },
		}))
		let result: unknown
		function Test() {
			result = useRoomState(room, "m.room.name", "")
			return null
		}
		act(() => root.render(createElement(Test)))
		expect((result as { content: { name: string } }).content.name).toBe("Named")
	})

	test("useRoomState with undefined room/type returns null", () => {
		let result: unknown
		function Test() {
			result = useRoomState(undefined, undefined)
			return null
		}
		act(() => root.render(createElement(Test)))
		expect(result).toBeNull()
	})

	test("useMultipleRoomMembers requests missing member events", () => {
		const client = new Client()
		const { room } = makeStoreAndRoom()
		room.applyState(makeEvent({
			rowid: 1, type: "m.room.member", state_key: "@known:example.com",
			content: { membership: "join", displayname: "Known" },
		}))
		let result: [string, unknown][] | undefined
		function Test() {
			result = useMultipleRoomMembers(client, room, ["@known:example.com", "@unknown:example.com"])
			return null
		}
		act(() => root.render(createElement(Test)))
		expect(client.requestMemberEvent).toHaveBeenCalledWith(room, "@unknown:example.com")
		expect(result!.length).toBe(2)
		expect(result![0][1]).toEqual({ membership: "join", displayname: "Known" })
		expect(result![1][1]).toBeNull()
	})

	test("useRoomMembers returns member list", () => {
		const { room } = makeStoreAndRoom()
		room.applyState(makeEvent({
			rowid: 1, type: "m.room.member", state_key: "@known:example.com",
			content: { membership: "join" },
		}))
		let result: unknown
		function Test() {
			result = useRoomMembers(room)
			return null
		}
		act(() => root.render(createElement(Test)))
		expect((result as unknown[]).length).toBe(1)
	})

	test("useRoomMembers with undefined room returns empty array", () => {
		let result: unknown
		function Test() {
			result = useRoomMembers(undefined)
			return null
		}
		act(() => root.render(createElement(Test)))
		expect(result).toEqual([])
	})

	test("useRoomEvent returns event by ID", () => {
		const { room } = makeStoreAndRoom()
		room.applyEvent(makeEvent())
		let result: unknown
		function Test() {
			result = useRoomEvent(room, "$evt:example.com")
			return null
		}
		act(() => root.render(createElement(Test)))
		expect((result as { rowid: number }).rowid).toBe(1)
	})

	test("useRoomEvent with null ID returns null", () => {
		const { room } = makeStoreAndRoom()
		let result: unknown
		function Test() {
			result = useRoomEvent(room, null)
			return null
		}
		act(() => root.render(createElement(Test)))
		expect(result).toBeNull()
	})

	test("useAccountData returns account data", () => {
		const { store } = makeStoreAndRoom()
		store.accountData.set("test.type", { value: 42 })
		let result: unknown
		function Test() {
			result = useAccountData(store, "test.type")
			return null
		}
		act(() => root.render(createElement(Test)))
		expect(result).toEqual({ value: 42 })
	})

	test("useAccountData with undefined type returns null", () => {
		const { store } = makeStoreAndRoom()
		let result: unknown
		function Test() {
			result = useAccountData(store, undefined)
			return null
		}
		act(() => root.render(createElement(Test)))
		expect(result).toBeNull()
	})

	test("useRoomAccountData returns room account data", () => {
		const { room } = makeStoreAndRoom()
		room.accountData.set("test.type", { value: 1 })
		let result: unknown
		function Test() {
			result = useRoomAccountData(room, "test.type")
			return null
		}
		act(() => root.render(createElement(Test)))
		expect(result).toEqual({ value: 1 })
	})

	test("useRoomAccountData with null room or undefined type returns null", () => {
		let result1: unknown, result2: unknown
		function Test() {
			result1 = useRoomAccountData(null, "test.type")
			result2 = useRoomAccountData(null, undefined)
			return null
		}
		act(() => root.render(createElement(Test)))
		expect(result1).toBeNull()
		expect(result2).toBeNull()
	})

	test("useSpaceEdges returns children", () => {
		const { store } = makeStoreAndRoom()
		const space = store.getSpaceStore("!space:example.com", true)
		space.children = [{ child_id: "!child:example.com" }]
		let result: unknown
		function Test() {
			result = useSpaceEdges(space)
			return null
		}
		act(() => root.render(createElement(Test)))
		expect(result).toEqual([{ child_id: "!child:example.com" }])
	})

	test("useSpaceEdges with undefined store returns null", () => {
		let result: unknown
		function Test() {
			result = useSpaceEdges(undefined)
			return null
		}
		act(() => root.render(createElement(Test)))
		expect(result).toBeNull()
	})

	test("usePreferences subscribes to preference changes", () => {
		const { store, room } = makeStoreAndRoom()
		let rendered = 0
		function Test() {
			usePreferences(store, room)
			rendered++
			return null
		}
		act(() => root.render(createElement(Test)))
		expect(rendered).toBe(1)
		act(() => store.preferenceSub.notify())
		expect(rendered).toBe(2)
	})

	test("usePreference returns current value and updates on change", () => {
		const { store } = makeStoreAndRoom()
		let current: unknown
		function Test() {
			current = usePreference(store, null, "pin_favorites")
			return null
		}
		act(() => root.render(createElement(Test)))
		expect(current).toBe(false)
		act(() => {
			store.localPreferenceCache.pin_favorites = true
		})
		expect(current).toBe(true)
	})

	test("usePreference falls back to room preference", () => {
		const { store, room } = makeStoreAndRoom()
		room.serverPreferenceCache.send_typing_notifications = false
		let current: unknown
		function Test() {
			current = usePreference(store, room, "send_typing_notifications")
			return null
		}
		act(() => root.render(createElement(Test)))
		expect(current).toBe(false)
	})

	test("useCustomEmojis returns merged packs", () => {
		const { store, room } = makeStoreAndRoom()
		let result: unknown
		function Test() {
			result = useCustomEmojis(store, room)
			return null
		}
		act(() => root.render(createElement(Test)))
		expect(result).toEqual([])
	})

	test("useSubscribedPacks returns emoji pack keys", () => {
		const { store } = makeStoreAndRoom()
		let result: unknown
		function Test() {
			result = useSubscribedPacks(store)
			return null
		}
		act(() => root.render(createElement(Test)))
		expect(result).toEqual([])
	})

	test("useRoomImagePacks returns room packs", () => {
		const { room } = makeStoreAndRoom()
		let result: unknown
		function Test() {
			result = useRoomImagePacks(room)
			return null
		}
		act(() => root.render(createElement(Test)))
		expect(result).toEqual({})
	})

	test("useBotCommands returns commands", () => {
		const { room } = makeStoreAndRoom()
		let result: unknown
		function Test() {
			result = useBotCommands(room)
			return null
		}
		act(() => root.render(createElement(Test)))
		expect(Array.isArray(result)).toBe(true)
		expect((result as unknown[]).length).toBeGreaterThan(0)
	})

	test("useReadReceipts returns receipts and updates on change", () => {
		const { room } = makeStoreAndRoom()
		room.applyEvent(makeEvent({ rowid: 1, timeline_rowid: 10 }))
		room.applyReceipts(
				[{ user_id: "@a:example.com", receipt_type: "m.read", event_id: "$evt:example.com", timestamp: 1 }],
				"$evt:example.com", false,
		)
		let result: unknown
		function Test() {
			result = useReadReceipts(room, "$evt:example.com")
			return null
		}
		act(() => root.render(createElement(Test)))
		expect((result as unknown[]).length).toBe(1)
	})

	test("useReadReceipts merges receipts from extra events", () => {
		const { room } = makeStoreAndRoom()
		room.applyEvent(makeEvent({ rowid: 1, timeline_rowid: 10 }))
		room.applyEvent(makeEvent({ rowid: 2, timeline_rowid: 20, event_id: "$evt2:example.com" }))
		room.applyReceipts(
				[{ user_id: "@a:example.com", receipt_type: "m.read", event_id: "$evt:example.com", timestamp: 1 }],
				"$evt:example.com", false,
		)
		room.applyReceipts(
				[{ user_id: "@b:example.com", receipt_type: "m.read", event_id: "$evt2:example.com", timestamp: 2 }],
				"$evt2:example.com", false,
		)
		let result: unknown
		function Test() {
			result = useReadReceipts(room, "$evt:example.com", ["$evt2:example.com"])
			return null
		}
		act(() => root.render(createElement(Test)))
		expect((result as unknown[]).length).toBe(2)
	})

	test("useReadReceipts updates when receipts change", () => {
		const { room } = makeStoreAndRoom()
		room.applyEvent(makeEvent({ rowid: 1, timeline_rowid: 10 }))
		let result: unknown
		function Test() {
			result = useReadReceipts(room, "$evt:example.com")
			return null
		}
		act(() => root.render(createElement(Test)))
		expect((result as unknown[]).length).toBe(0)
		act(() => {
			room.applyReceipts(
					[{ user_id: "@a:example.com", receipt_type: "m.read", event_id: "$evt:example.com", timestamp: 1 }],
				"$evt:example.com", false,
			)
		})
		expect((result as unknown[]).length).toBe(1)
	})

	test("useRoomMember returns member event when present", () => {
		const client = new Client()
		const { room } = makeStoreAndRoom()
		room.applyState(makeEvent({
			rowid: 1, type: "m.room.member", state_key: "@known:example.com",
			content: { membership: "join", displayname: "Known" },
		}))
		let result: unknown
		function Test() {
			result = useRoomMember(client, room, "@known:example.com")
			return null
		}
		act(() => root.render(createElement(Test)))
		expect((result as { content: { displayname: string } }).content.displayname).toBe("Known")
	})

	test("useRoomMember requests missing member event", () => {
		const client = new Client()
		const { room } = makeStoreAndRoom()
		let result: unknown
		function Test() {
			result = useRoomMember(client, room, "@missing:example.com")
			return null
		}
		act(() => root.render(createElement(Test)))
		expect(client.requestMemberEvent).toHaveBeenCalledWith(room, "@missing:example.com")
		expect(result).toBeNull()
	})

	test("useRoomMember with null client does not request", () => {
		const { room } = makeStoreAndRoom()
		let result: unknown
		function Test() {
			result = useRoomMember(null, room, "@missing:example.com")
			return null
		}
		act(() => root.render(createElement(Test)))
		expect(result).toBeNull()
	})

	test("usePreferences with null room works", () => {
		const { store } = makeStoreAndRoom()
		let rendered = 0
		function Test() {
			usePreferences(store, null)
			rendered++
			return null
		}
		act(() => root.render(createElement(Test)))
		expect(rendered).toBe(1)
	})

	test("usePreference with null store returns default", () => {
		let current: unknown
		function Test() {
			current = usePreference(null, null, "pin_favorites")
			return null
		}
		act(() => root.render(createElement(Test)))
		expect(current).toBe(false)
	})

	test("useCustomEmojis with stickers usage", () => {
		const { store, room } = makeStoreAndRoom()
		let result: unknown
		function Test() {
			result = useCustomEmojis(store, room, "stickers")
			return null
		}
		act(() => root.render(createElement(Test)))
		expect(result).toEqual([])
	})
})
