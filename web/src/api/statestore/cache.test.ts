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
import { DBAccountData, DBRoom, RoomNameQuality } from "../types"
import StateCache from "./cache.ts"

// ---------------------------------------------------------------------------
// Minimal in-memory IndexedDB fake. Callbacks fire via Promise.resolve() so
// they land after the synchronous assignment window but before the next await.
// Do NOT use vi.useFakeTimers() in these tests — it intercepts queueMicrotask.
// ---------------------------------------------------------------------------

function nextTick() {
	return Promise.resolve()
}

class FakeIDBRequest<T = unknown> {
	result: T = undefined as never
	error: Error | null = null
	onsuccess: ((evt: { target: FakeIDBRequest<T> }) => void) | null = null
	onerror: ((evt: unknown) => void) | null = null
	onblocked: ((evt: unknown) => void) | null = null
	onupgradeneeded: ((evt: { oldVersion: number }) => void) | null = null

	succeed(result: T) {
		this.result = result
		nextTick().then(() => this.onsuccess?.({ target: this }))
	}
}

class FakeObjectStore {
	records = new Map<string, unknown>()

	constructor(public name: string) {}

	private static key(key: string | string[] | unknown): string {
		return Array.isArray(key) ? JSON.stringify(key) : `${key}`
	}

	get(key: string | string[]): FakeIDBRequest {
		const req = new FakeIDBRequest()
		req.succeed(this.records.get(FakeObjectStore.key(key)))
		return req
	}

	getAll(): FakeIDBRequest {
		const req = new FakeIDBRequest()
		req.succeed([...this.records.values()])
		return req
	}

	put(value: Record<string, unknown>, key?: string | string[]): FakeIDBRequest {
		const req = new FakeIDBRequest()
		const realKey = key ?? (value.key ?? value.room_id ?? value.type) as string
		this.records.set(FakeObjectStore.key(realKey), value)
		req.succeed(value)
		return req
	}

	delete(key: string): FakeIDBRequest {
		const req = new FakeIDBRequest()
		this.records.delete(FakeObjectStore.key(key))
		req.succeed(undefined)
		return req
	}

	clear(): FakeIDBRequest {
		const req = new FakeIDBRequest()
		this.records.clear()
		req.succeed(undefined)
		return req
	}
}

class FakeTransaction {
	oncomplete: (() => void) | null = null
	onerror: (() => void) | null = null
	onabort: (() => void) | null = null
	error: Error | null = null
	private done = false
	shouldFail = false

	constructor(private storeMap: Map<string, FakeObjectStore>) {
		// Auto-commit after callers get their request results
		nextTick().then(() => nextTick()).then(() => nextTick()).then(() => this.finish())
	}

	objectStore(name: string): FakeObjectStore {
		const store = this.storeMap.get(name)
		if (!store) {
			throw new Error(`No such store: ${name}`)
		}
		return store
	}

	commit() {
		this.finish()
	}

	abort() {
		this.finish(new Error("Aborted"))
	}

	finish(err: Error | null = null) {
		if (this.done) {
			return
		}
		this.done = true
		this.error = err ?? (this.shouldFail ? new Error("Simulated failure") : null)
		nextTick().then(() => {
			if (this.error) {
				this.onerror?.()
			} else {
				this.oncomplete?.()
			}
		})
	}
}

const STORE_NAMES = [
	"kv_store", "invited_room", "room", "space_edges", "account_data", "room_account_data",
]

class FakeDatabase {
	version = 2
	onversionchange: (() => void) | null = null
	closed = false
	stores = new Map<string, FakeObjectStore>()
	failNextTransaction = false

	constructor(public name: string) {
		for (const store of STORE_NAMES) {
			this.stores.set(store, new FakeObjectStore(store))
		}
	}

	createObjectStore(name: string): FakeObjectStore {
		const store = new FakeObjectStore(name)
		this.stores.set(name, store)
		return store
	}

	deleteObjectStore(name: string) {
		this.stores.delete(name)
	}

	transaction(storeNames: string[]): FakeTransaction {
		const storeMap = new Map(storeNames.map(n => [n, this.stores.get(n)!]))
		const txn = new FakeTransaction(storeMap)
		if (this.failNextTransaction) {
			this.failNextTransaction = false
			txn.shouldFail = true
		}
		return txn
	}

	close() {
		this.closed = true
	}
}

const dbRegistry = new Map<string, FakeDatabase>()

const fakeIndexedDB = {
	open(name: string, requestedVersion?: number): FakeIDBRequest<FakeDatabase> {
		const req = new FakeIDBRequest<FakeDatabase>()
		nextTick().then(() => {
			let db = dbRegistry.get(name)
			const isNew = !db
			if (!db) {
				db = new FakeDatabase(name)
				db.version = 0
				dbRegistry.set(name, db)
			}
			const upgradeNeeded = requestedVersion !== undefined && db.version < requestedVersion
			// result must be set BEFORE onupgradeneeded fires (cache.ts creates
			// object stores inside the handler via req.result.createObjectStore)
			req.result = db
			if (upgradeNeeded) {
				const oldVersion = db.version
				db.version = requestedVersion
				req.onupgradeneeded?.({ oldVersion })
			}
			req.onsuccess?.({ target: req })
		})
		return req
	},

	deleteDatabase(name: string, fireBlocked = false): FakeIDBRequest {
		const req = new FakeIDBRequest()
		dbRegistry.delete(name)
		if (fireBlocked) {
			nextTick().then(() => req.onblocked?.({ target: req }))
		}
		req.succeed(undefined)
		return req
	},
}

declare global {
	interface Window {
		indexedDB: typeof fakeIndexedDB
	}
}

beforeEach(() => {
	dbRegistry.clear()
	window.indexedDB = fakeIndexedDB
})

afterEach(() => {
	vi.restoreAllMocks()
})

function makeAccountData(): DBAccountData {
	return {
		user_id: "@user:example.com",
		type: "test.type",
		content: { value: 1 },
	}
}

function makeMeta(): DBRoom {
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
	}
}

describe("StateCache", () => {
	test("load with empty db returns null", async () => {
		const cache = new StateCache()
		const data = await cache.load()
		expect(data).toBeNull()
		cache.close()
	})

	test("setters queue updates flushed to db; load returns them on next open", async () => {
		const cache = new StateCache()
		await cache.load()
		cache.setUserID("@user:example.com")
		cache.setServerTimestamp(12345)
		cache.setTopLevelSpaces(["!space:example.com"])
		cache.setAccountData(makeAccountData())
		cache.setSpaceEdges("!space:example.com", [{ child_id: "!room:example.com" }])
		cache.setInvitedRoom({ room_id: "!invite:example.com", created_at: 0, invite_state: [] })
		cache.setRoomAccountData({
			user_id: "@user:example.com",
			room_id: "!room:example.com",
			type: "test.room.type",
			content: {},
		})
		cache.setRoom({ meta: makeMeta(), state: {}, events: [] })
		cache.deleteRoom("!other:example.com")
		cache.deleteInvitedRoom("!gone:example.com")
		await cache.tryFlush()
		cache.close()

		const cache2 = new StateCache()
		const data = await cache2.load()
		expect(data?.user_id).toBe("@user:example.com")
		expect(data?.server_timestamp).toBe(12345)
		expect(data?.top_level_spaces).toEqual(["!space:example.com"])
		expect(data?.space_edges?.["!space:example.com"]).toEqual([{ child_id: "!room:example.com" }])
		cache2.close()
	})

	test("load merges room_account_data into rooms", async () => {
		const cache = new StateCache()
		await cache.load()
		cache.setUserID("@user:example.com")
		cache.setServerTimestamp(999)
		cache.setTopLevelSpaces([])
		cache.setAccountData(makeAccountData())
		cache.setRoom({ meta: makeMeta(), state: {}, events: [] })
		cache.setRoomAccountData({
			user_id: "@user:example.com",
			room_id: "!room:example.com",
			type: "m.tag",
			content: { tags: {} },
		})
		await cache.tryFlush()
		cache.close()

		const cache2 = new StateCache()
		const data = await cache2.load()
		expect(data!.rooms["!room:example.com"]).toBeDefined()
		expect(data!.rooms["!room:example.com"].account_data?.["m.tag"]).toBeDefined()
		expect(data!.account_data["test.type"]).toBeDefined()
		expect(data!.clear_state).toBe(true)
		cache2.close()
	})

	test("tryFlush with nothing queued resolves immediately", async () => {
		const cache = new StateCache()
		await cache.load()
		await cache.tryFlush()
		cache.close()
	})

	test("tryFlush after close logs error without throwing", async () => {
		const cache = new StateCache()
		await cache.load()
		cache.close()
		await cache.tryFlush()
	})

	test("setters are no-ops before load (no db)", () => {
		const cache = new StateCache()
		expect(() => {
			cache.setUserID("@user:example.com")
			cache.setRoom({ meta: makeMeta(), state: {}, events: [] })
		}).not.toThrow()
	})

	test("clear wipes all stores", async () => {
		const cache = new StateCache()
		await cache.load()
		cache.setUserID("@user:example.com")
		cache.setServerTimestamp(1)
		cache.setTopLevelSpaces([])
		cache.setAccountData(makeAccountData())
		await cache.tryFlush()
		await cache.clear()
		cache.close()
		const cache2 = new StateCache()
		const data = await cache2.load()
		expect(data).toBeNull()
		cache2.close()
	})

	test("clear before load resolves immediately", async () => {
		const cache = new StateCache()
		await cache.clear()
	})

	test("delete removes the database", async () => {
		const cache = new StateCache()
		await cache.load()
		cache.close()
		await StateCache.delete()
		expect(dbRegistry.size).toBe(0)
	})

	test("close stops flush interval and sets db to undefined", async () => {
		const clearIntervalSpy = vi.spyOn(globalThis, "clearInterval")
		const cache = new StateCache()
		await cache.load()
		cache.close()
		expect(clearIntervalSpy).toHaveBeenCalled()
		expect(cache["db"]).toBeUndefined()
	})

	test("tryFlush already flushing resolves with rejection", async () => {
		const cache = new StateCache()
		await cache.load()
		cache.setAccountData(makeAccountData())
		// Trigger double-flush scenario: start first flush (pending), start second before it completes
		const p1 = cache["flush"]()
		const p2 = cache["flush"]()
		const result2 = await p2.catch(e => e.message)
		expect(result2).toContain("Already flushing")
		await p1
		cache.close()
	})

	test("flush restores queue when transaction fails", async () => {
		const cache = new StateCache()
		await cache.load()
		const db = cache["db"] as unknown as FakeDatabase
		cache.setAccountData(makeAccountData())
		db.failNextTransaction = true
		const result = await cache.tryFlush()
		expect(result).toBeUndefined()
		// The failed flush should have restored the queue
		expect(cache["queue"].size).toBe(1)
		cache.close()
	})

	test("load fails when read transaction errors", async () => {
		// Pre-create the db with data so load() reaches the read transaction
		const seed = new StateCache()
		await seed.load()
		seed.setUserID("@user:example.com")
		seed.setServerTimestamp(1)
		seed.setTopLevelSpaces([])
		seed.setAccountData(makeAccountData())
		await seed.tryFlush()
		seed.close()

		const db = dbRegistry.get("gomuks-cache")!
		db.failNextTransaction = true
		const cache = new StateCache()
		await expect(cache.load()).rejects.toThrow("Simulated failure")
	})

	test("clear rejects when transaction errors", async () => {
		const cache = new StateCache()
		await cache.load()
		const db = cache["db"] as unknown as FakeDatabase
		db.failNextTransaction = true
		await expect(cache.clear()).rejects.toThrow("Simulated failure")
		cache.close()
	})

	test("onversionchange closes the cache", async () => {
		const cache = new StateCache()
		await cache.load()
		const db = cache["db"] as unknown as FakeDatabase
		expect(db.onversionchange).toBeTruthy()
		db.onversionchange!()
		expect(db.closed).toBe(true)
		expect(cache["db"]).toBeUndefined()
	})

	test("upgrade from version 1 deletes old stores and recreates them", async () => {
		// Seed a legacy v1 database with only the old stores
		const legacy = new FakeDatabase("gomuks-cache")
		legacy.version = 1
		legacy.stores.clear()
		legacy.stores.set("kv_store", new FakeObjectStore("kv_store"))
		dbRegistry.set("gomuks-cache", legacy)

		const cache = new StateCache()
		const data = await cache.load()
		expect(data).toBeNull()
		expect(legacy.stores.has("room_account_data")).toBe(true)
		cache.close()
	})
})
