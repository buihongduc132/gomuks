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
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { getLocalStoragePreferences } from "./localstorage.ts"
import { preferences } from "./preferences.ts"

describe("getLocalStoragePreferences", () => {
	beforeEach(() => {
		localStorage.clear()
	})

	afterEach(() => {
		localStorage.clear()
	})

	it("returns empty preferences when localStorage is empty", () => {
		const onChange = vi.fn()
		const prefs = getLocalStoragePreferences("global_prefs", onChange)
		expect(prefs.send_read_receipts).toBeUndefined()
		expect(onChange).not.toHaveBeenCalled()
	})

	it("reads existing preferences from localStorage", () => {
		localStorage.setItem("global_prefs", JSON.stringify({ send_read_receipts: false }))
		const onChange = vi.fn()
		const prefs = getLocalStoragePreferences("global_prefs", onChange)
		expect(prefs.send_read_receipts).toBe(false)
	})

	it("returns empty object when localStorage has invalid JSON", () => {
		localStorage.setItem("global_prefs", "not valid json")
		const onChange = vi.fn()
		const prefs = getLocalStoragePreferences("global_prefs", onChange)
		expect(prefs.send_read_receipts).toBeUndefined()
	})

	it("set trap updates target and localStorage, calls onChange", () => {
		const onChange = vi.fn()
		const prefs = getLocalStoragePreferences("global_prefs", onChange)
		;(prefs as Record<string, unknown>).send_read_receipts = false
		expect(prefs.send_read_receipts).toBe(false)
		const stored = JSON.parse(localStorage.getItem("global_prefs")!)
		expect(stored.send_read_receipts).toBe(false)
		expect(onChange).toHaveBeenCalledTimes(1)
	})

	it("set trap rejects invalid preference keys", () => {
		const onChange = vi.fn()
		const prefs = getLocalStoragePreferences("global_prefs", onChange)
		const result = Reflect.set(prefs as Record<string, unknown>, "nonexistent_key", "value")
		expect(result).toBe(false)
		expect(onChange).not.toHaveBeenCalled()
	})

	it("deleteProperty removes key and updates localStorage", () => {
		localStorage.setItem("global_prefs", JSON.stringify({
			send_read_receipts: false,
			send_typing_notifications: false,
		}))
		const onChange = vi.fn()
		const prefs = getLocalStoragePreferences("global_prefs", onChange)
		const result = Reflect.deleteProperty(prefs as Record<string, unknown>, "send_read_receipts")
		expect(result).toBe(true)
		expect(prefs.send_read_receipts).toBeUndefined()
		const stored = JSON.parse(localStorage.getItem("global_prefs")!)
		expect(stored.send_read_receipts).toBeUndefined()
		expect(stored.send_typing_notifications).toBe(false)
		expect(onChange).toHaveBeenCalledTimes(1)
	})

	it("deleteProperty removes localStorage key when object becomes empty", () => {
		localStorage.setItem("global_prefs", JSON.stringify({ send_read_receipts: false }))
		const onChange = vi.fn()
		const prefs = getLocalStoragePreferences("global_prefs", onChange)
		Reflect.deleteProperty(prefs as Record<string, unknown>, "send_read_receipts")
		expect(localStorage.getItem("global_prefs")).toBeNull()
		expect(onChange).toHaveBeenCalledTimes(1)
	})

	it("deleteProperty rejects invalid keys", () => {
		const onChange = vi.fn()
		const prefs = getLocalStoragePreferences("global_prefs", onChange)
		const result = Reflect.deleteProperty(prefs as Record<string, unknown>, "nonexistent_key")
		expect(result).toBe(false)
		expect(onChange).not.toHaveBeenCalled()
	})

	it("ownKeys returns globalPrefKeys for 'global_prefs'", () => {
		const onChange = vi.fn()
		const prefs = getLocalStoragePreferences("global_prefs", onChange)
		const keys = Object.keys(prefs)
		// globalPrefKeys = keys where allowedContexts includes Device
		const expectedKeys = Object.entries(preferences)
			.filter(([, pref]) => pref.allowedContexts.includes("device" as never))
			.map(([key]) => key)
		expect(keys).toEqual(expectedKeys)
	})

	it("ownKeys returns roomPrefKeys for non-global keys", () => {
		const onChange = vi.fn()
		const prefs = getLocalStoragePreferences("prefs-!room:example.com", onChange)
		const keys = Object.keys(prefs)
		// roomPrefKeys = keys where allowedContexts includes RoomDevice
		const expectedKeys = Object.entries(preferences)
			.filter(([, pref]) => pref.allowedContexts.includes("room_device" as never))
			.map(([key]) => key)
		expect(keys).toEqual(expectedKeys)
	})

	it("getOwnPropertyDescriptor returns descriptor for valid keys", () => {
		const onChange = vi.fn()
		const prefs = getLocalStoragePreferences("global_prefs", onChange)
		const desc = Object.getOwnPropertyDescriptor(prefs, "send_read_receipts")
		expect(desc).toBeDefined()
		expect(desc?.enumerable).toBe(true)
		expect(desc?.configurable).toBe(true)
		expect(desc?.writable).toBe(true)
	})

	it("getOwnPropertyDescriptor returns undefined for invalid keys", () => {
		const onChange = vi.fn()
		const prefs = getLocalStoragePreferences("global_prefs", onChange)
		const desc = Object.getOwnPropertyDescriptor(prefs, "nonexistent_key")
		expect(desc).toBeUndefined()
	})
})
