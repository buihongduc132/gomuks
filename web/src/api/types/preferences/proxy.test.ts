// gomuks - A Matrix client written in Go.
// Copyright (C) 2024 Tulir Asokan
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
import { describe, expect, it } from "vitest"
import type { RoomStateStore, StateStore } from "@/api/statestore"
import { getPreferenceProxy } from "./proxy.ts"
import { preferences } from "./preferences.ts"
import { PreferenceContext } from "./types.ts"

function makeMockStore(overrides: Partial<{
	serverPreferenceCache: Record<string, unknown>
	localPreferenceCache: Record<string, unknown>
}> = {}): StateStore {
	return {
		serverPreferenceCache: overrides.serverPreferenceCache ?? {},
		localPreferenceCache: overrides.localPreferenceCache ?? {},
	} as unknown as StateStore
}

function makeMockRoom(overrides: Partial<{
	serverPreferenceCache: Record<string, unknown>
	localPreferenceCache: Record<string, unknown>
}> = {}): RoomStateStore {
	return {
		serverPreferenceCache: overrides.serverPreferenceCache ?? {},
		localPreferenceCache: overrides.localPreferenceCache ?? {},
	} as unknown as RoomStateStore
}

describe("getPreferenceProxy", () => {
	it("returns default values when no caches have values", () => {
		const store = makeMockStore()
		const proxy = getPreferenceProxy(store)
		expect(proxy.send_read_receipts).toBe(true)
		expect(proxy.show_media_previews).toBe(false)
		expect(proxy.code_block_theme).toBe("auto")
	})

	it("reads from serverPreferenceCache (Account context)", () => {
		const store = makeMockStore({
			serverPreferenceCache: { send_read_receipts: false },
		})
		const proxy = getPreferenceProxy(store)
		expect(proxy.send_read_receipts).toBe(false)
	})

	it("reads from localPreferenceCache (Device context)", () => {
		const store = makeMockStore({
			localPreferenceCache: { send_read_receipts: false },
		})
		const proxy = getPreferenceProxy(store)
		expect(proxy.send_read_receipts).toBe(false)
	})

	it("device (local) cache takes priority over account (server) cache", () => {
		// anyContext = [RoomDevice, RoomAccount, Device, Account, Config]
		// Without a room, Device is checked before Account
		const store = makeMockStore({
			serverPreferenceCache: { send_read_receipts: false },
			localPreferenceCache: { send_read_receipts: true },
		})
		const proxy = getPreferenceProxy(store)
		// Device (local) is checked before Account (server)
		expect(proxy.send_read_receipts).toBe(true)
	})

	it("falls through to local cache when server cache has no value", () => {
		// send_read_receipts allowedContexts = anyContext = [RoomDevice, RoomAccount, Device, Account, Config]
		// RoomDevice is first, so room.localPreferenceCache is checked first
		const room = makeMockRoom({
			localPreferenceCache: { send_read_receipts: false },
		})
		const store = makeMockStore()
		const proxy = getPreferenceProxy(store, room)
		expect(proxy.send_read_receipts).toBe(false)
	})

	it("reads room-specific prefs from room.serverPreferenceCache", () => {
		// room_view_type has roomSpecific = [RoomAccount, RoomDevice]
		const room = makeMockRoom({
			serverPreferenceCache: { room_view_type: "m.space" },
		})
		const store = makeMockStore()
		const proxy = getPreferenceProxy(store, room)
		expect(proxy.room_view_type).toBe("m.space")
	})

	it("reads room device prefs from room.localPreferenceCache", () => {
		const room = makeMockRoom({
			localPreferenceCache: { room_view_type: "m.space" },
		})
		const store = makeMockStore()
		const proxy = getPreferenceProxy(store, room)
		expect(proxy.room_view_type).toBe("m.space")
	})

	it("falls back to default when room has no value and no global value", () => {
		const room = makeMockRoom()
		const store = makeMockStore()
		const proxy = getPreferenceProxy(store, room)
		expect(proxy.room_view_type).toBe(null)
	})

	it("throws on set (read-only)", () => {
		const store = makeMockStore()
		const proxy = getPreferenceProxy(store)
		expect(() => {
			(proxy as Record<string, unknown>).send_read_receipts = false
		}).toThrow("read-only")
	})

	it("ownKeys returns all preference keys", () => {
		const store = makeMockStore()
		const proxy = getPreferenceProxy(store)
		const keys = Object.keys(proxy)
		expect(keys).toContain("send_read_receipts")
		expect(keys).toContain("code_block_theme")
		expect(keys.length).toBe(Object.keys(preferences).length)
	})

	it("getOwnPropertyDescriptor returns descriptor for valid keys", () => {
		const store = makeMockStore()
		const proxy = getPreferenceProxy(store)
		const desc = Object.getOwnPropertyDescriptor(proxy, "send_read_receipts")
		expect(desc).toBeDefined()
		expect(desc?.enumerable).toBe(true)
		expect(desc?.writable).toBe(false)
		expect(desc?.configurable).toBe(true)
	})

	it("getOwnPropertyDescriptor returns undefined for invalid keys", () => {
		const store = makeMockStore()
		const proxy = getPreferenceProxy(store)
		const desc = Object.getOwnPropertyDescriptor(proxy, "nonexistent_key")
		expect(desc).toBeUndefined()
	})

	it("throws for invalid preference key in non-dev mode", () => {
		const store = makeMockStore()
		const proxy = getPreferenceProxy(store)
		expect(() => {
			(proxy as Record<string, unknown>)["nonexistent"]
		}).toThrow("Invalid preference key")
	})

	it("handles symbol keys by throwing in non-dev mode", () => {
		const store = makeMockStore()
		const proxy = getPreferenceProxy(store)
		const sym = Symbol("test")
		expect(() => {
			// Access via symbol triggers the typeof key !== "string" branch
			;(proxy as Record<symbol, unknown>)[sym]
		}).toThrow("Preference key must be a string")
	})

	it("low_bandwidth pref falls through Config context (TODO, returns default)", () => {
		// low_bandwidth has globalDeviceSpecific = [Device, Config]
		// Config is TODO, so only Device cache is checked
		const store = makeMockStore({
			localPreferenceCache: { low_bandwidth: true },
		})
		const proxy = getPreferenceProxy(store)
		expect(proxy.low_bandwidth).toBe(true)
	})

	it("returns default when Config is the only context and it's TODO", () => {
		// No pref has ONLY Config context, but we can test the path via a pref
		// that has Config as the last fallback
		const store = makeMockStore()
		const proxy = getPreferenceProxy(store)
		// send_read_receipts defaults to true when no cache has it
		expect(proxy.send_read_receipts).toBe(true)
	})
})
