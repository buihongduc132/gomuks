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
import { describe, expect, test, vi } from "vitest"
import type { RoomListEntry } from "./main.ts"
import {
	DirectChatSpace,
	HomeSpace,
	SpaceEdgeStore,
	SpaceOrphansSpace,
	UnreadsSpace,
} from "./space.ts"

function makeEntry(overrides: Partial<RoomListEntry> = {}): RoomListEntry {
	return {
		room_id: "!room:example.com",
		sorting_timestamp: 1000,
		name: "Test Room",
		search_name: "test room",
		unread_messages: 0,
		unread_notifications: 0,
		unread_highlights: 0,
		marked_unread: false,
		...overrides,
	}
}

// Minimal StateStore mock for SpaceEdgeStore parent
function makeMockStore(spaceStores: Record<string, SpaceEdgeStore> = {}) {
	return {
		getSpaceStore: vi.fn((id: string) => spaceStores[id] ?? null),
		activeRoomID: null as string | null,
	} as unknown as ConstructorParameters<typeof SpaceEdgeStore>[1]
}

describe("HomeSpace", () => {
	test("include always returns true", () => {
		const space = new HomeSpace()
		expect(space.include(makeEntry())).toBe(true)
		expect(space.include(makeEntry({ room_id: "!other:example.com" }))).toBe(true)
	})

	test("has id", () => {
		expect(new HomeSpace().id).toBe("fi.mau.gomuks.home")
	})
})

describe("DirectChatSpace", () => {
	test("include returns true for rooms with dm_user_id", () => {
		const space = new DirectChatSpace()
		expect(space.include(makeEntry({ dm_user_id: "@user:example.com" }))).toBe(true)
	})

	test("include returns false for rooms without dm_user_id", () => {
		const space = new DirectChatSpace()
		expect(space.include(makeEntry())).toBe(false)
		expect(space.include(makeEntry({ dm_user_id: undefined }))).toBe(false)
	})
})

describe("UnreadsSpace", () => {
	test("include returns true for active room", () => {
		const store = makeMockStore()
		;(store as unknown as { activeRoomID: string | null }).activeRoomID = "!active:example.com"
		const space = new UnreadsSpace(store as never)
		expect(space.include(makeEntry({ room_id: "!active:example.com" }))).toBe(true)
	})

	test("include returns true for rooms with unread messages", () => {
		const space = new UnreadsSpace(makeMockStore() as never)
		expect(space.include(makeEntry({ unread_messages: 5 }))).toBe(true)
	})

	test("include returns true for rooms with unread notifications", () => {
		const space = new UnreadsSpace(makeMockStore() as never)
		expect(space.include(makeEntry({ unread_notifications: 2 }))).toBe(true)
	})

	test("include returns true for rooms with unread highlights", () => {
		const space = new UnreadsSpace(makeMockStore() as never)
		expect(space.include(makeEntry({ unread_highlights: 1 }))).toBe(true)
	})

	test("include returns true for marked unread rooms", () => {
		const space = new UnreadsSpace(makeMockStore() as never)
		expect(space.include(makeEntry({ marked_unread: true }))).toBe(true)
	})

	test("include returns false for read rooms", () => {
		const space = new UnreadsSpace(makeMockStore() as never)
		expect(space.include(makeEntry())).toBe(false)
	})
})

describe("Space unread counts", () => {
	test("applyUnreads adds new counts", () => {
		const space = new HomeSpace()
		space.applyUnreads({ unread_messages: 5, unread_notifications: 2, unread_highlights: 1 })
		expect(space.counts.current).toEqual({
			unread_messages: 5,
			unread_notifications: 2,
			unread_highlights: 1,
		})
	})

	test("applyUnreads subtracts old counts", () => {
		const space = new HomeSpace()
		space.applyUnreads({ unread_messages: 5, unread_notifications: 2, unread_highlights: 1 })
		space.applyUnreads(
			{ unread_messages: 3, unread_notifications: 1, unread_highlights: 0 },
			{ unread_messages: 5, unread_notifications: 2, unread_highlights: 1 },
		)
		expect(space.counts.current).toEqual({
			unread_messages: 3,
			unread_notifications: 1,
			unread_highlights: 0,
		})
	})

	test("applyUnreads clamps negative counts to zero", () => {
		const space = new HomeSpace()
		space.applyUnreads(null, { unread_messages: 5, unread_notifications: 3, unread_highlights: 2 })
		expect(space.counts.current).toEqual({
			unread_messages: 0,
			unread_notifications: 0,
			unread_highlights: 0,
		})
	})

	test("applyUnreads with null newCounts and null oldCounts does nothing", () => {
		const space = new HomeSpace()
		space.applyUnreads(null, null)
		expect(space.counts.current).toEqual({
			unread_messages: 0,
			unread_notifications: 0,
			unread_highlights: 0,
		})
	})

	test("applyUnreads does not emit when counts unchanged", () => {
		const space = new HomeSpace()
		const listener = vi.fn()
		space.counts.listen(listener)
		listener.mockClear()
		space.applyUnreads({ unread_messages: 0, unread_notifications: 0, unread_highlights: 0 })
		expect(listener).not.toHaveBeenCalled()
	})

	test("clearUnreads resets counts to zero", () => {
		const space = new HomeSpace()
		space.applyUnreads({ unread_messages: 5, unread_notifications: 2, unread_highlights: 1 })
		space.clearUnreads()
		expect(space.counts.current).toEqual({
			unread_messages: 0,
			unread_notifications: 0,
			unread_highlights: 0,
		})
	})
})

describe("SpaceEdgeStore", () => {
	test("constructor sets id", () => {
		const store = new SpaceEdgeStore("!space:example.com", makeMockStore())
		expect(store.id).toBe("!space:example.com")
	})

	test("include returns false initially", () => {
		const store = new SpaceEdgeStore("!space:example.com", makeMockStore())
		expect(store.include(makeEntry({ room_id: "!room:example.com" }))).toBe(false)
	})

	test("setting children adds rooms to flattened set", () => {
		const store = new SpaceEdgeStore("!space:example.com", makeMockStore())
		store.children = [{ child_id: "!room:example.com" }]
		expect(store.include(makeEntry({ room_id: "!room:example.com" }))).toBe(true)
		expect(store.include(makeEntry({ room_id: "!other:example.com" }))).toBe(false)
	})

	test("replacing children updates flattened set", () => {
		const store = new SpaceEdgeStore("!space:example.com", makeMockStore())
		store.children = [{ child_id: "!room:example.com" }]
		store.children = [{ child_id: "!other:example.com" }]
		expect(store.include(makeEntry({ room_id: "!room:example.com" }))).toBe(false)
		expect(store.include(makeEntry({ room_id: "!other:example.com" }))).toBe(true)
	})

	test("child spaces get flattened rooms from children", () => {
		const childStore = new SpaceEdgeStore("!child:example.com", makeMockStore())
		const parentStore = new SpaceEdgeStore(
			"!parent:example.com",
			makeMockStore({ "!child:example.com": childStore }),
		)
		childStore.children = [{ child_id: "!room:example.com" }]
		parentStore.children = [{ child_id: "!child:example.com" }]
		expect(parentStore.include(makeEntry({ room_id: "!room:example.com" }))).toBe(true)
		expect(parentStore.children).toEqual([{ child_id: "!child:example.com" }])
	})

	test("childSpaces getter returns child spaces", () => {
		const childStore = new SpaceEdgeStore("!child:example.com", makeMockStore())
		const parentStore = new SpaceEdgeStore(
			"!parent:example.com",
			makeMockStore({ "!child:example.com": childStore }),
		)
		parentStore.children = [{ child_id: "!child:example.com" }]
		expect(parentStore.childSpaces.has(childStore)).toBe(true)
	})

	test("removing a child space from children removes parent relationship", () => {
		const childStore = new SpaceEdgeStore("!child:example.com", makeMockStore())
		const parentStore = new SpaceEdgeStore(
			"!parent:example.com",
			makeMockStore({ "!child:example.com": childStore }),
		)
		parentStore.children = [{ child_id: "!child:example.com" }]
		expect(parentStore.childSpaces.size).toBe(1)
		parentStore.children = []
		expect(parentStore.childSpaces.size).toBe(0)
	})

	test("nested space flattening propagates to grandparents", () => {
		const grandchild = new SpaceEdgeStore("!grandchild:example.com", makeMockStore())
		const child = new SpaceEdgeStore(
			"!child:example.com",
			makeMockStore({ "!grandchild:example.com": grandchild }),
		)
		const parent = new SpaceEdgeStore(
			"!parent:example.com",
			makeMockStore({ "!child:example.com": child }),
		)
		grandchild.children = [{ child_id: "!deep:example.com" }]
		child.children = [{ child_id: "!grandchild:example.com" }]
		parent.children = [{ child_id: "!child:example.com" }]
		expect(parent.include(makeEntry({ room_id: "!deep:example.com" }))).toBe(true)
	})

	test("sub subscriber gets notified on children change", () => {
		const store = new SpaceEdgeStore("!space:example.com", makeMockStore())
		const listener = vi.fn()
		store.sub.subscribe(listener)
		store.children = [{ child_id: "!room:example.com" }]
		expect(listener).toHaveBeenCalled()
	})

	test("applyUnreads works on SpaceEdgeStore", () => {
		const store = new SpaceEdgeStore("!space:example.com", makeMockStore())
		store.applyUnreads({ unread_messages: 10, unread_notifications: 0, unread_highlights: 0 })
		expect(store.counts.current.unread_messages).toBe(10)
	})
})

describe("SpaceOrphansSpace", () => {
	test("include returns true for rooms not in any space and not DMs", () => {
		const orphan = new SpaceOrphansSpace(makeMockStore() as never)
		expect(orphan.include(makeEntry({ room_id: "!orphan:example.com" }))).toBe(true)
	})

	test("include returns false for DM rooms", () => {
		const orphan = new SpaceOrphansSpace(makeMockStore() as never)
		expect(orphan.include(makeEntry({ room_id: "!orphan:example.com", dm_user_id: "@x:example.com" }))).toBe(false)
	})

	test("static id is set", () => {
		expect(SpaceOrphansSpace.id).toBe("fi.mau.gomuks.space_orphans")
	})
})
