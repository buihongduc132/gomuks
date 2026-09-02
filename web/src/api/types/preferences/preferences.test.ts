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
import {
	codeBlockStyles,
	existingPreferenceKeys,
	gifProviders,
	isValidPreferenceKey,
	mapProviders,
	preferences,
} from "./preferences.ts"

describe("constants", () => {
	it("codeBlockStyles is a non-empty readonly array", () => {
		expect(codeBlockStyles.length).toBeGreaterThan(0)
		expect(codeBlockStyles).toContain("auto")
		expect(codeBlockStyles).toContain("monokai")
	})

	it("mapProviders contains expected values", () => {
		expect(mapProviders).toEqual(["leaflet", "google", "none"])
	})

	it("gifProviders contains expected values", () => {
		expect(gifProviders).toEqual(["giphy", "tenor", "klipy"])
	})
})

describe("preferences object", () => {
	it("contains all expected preference keys", () => {
		const keys = Object.keys(preferences)
		expect(keys).toContain("send_read_receipts")
		expect(keys).toContain("send_typing_notifications")
		expect(keys).toContain("show_media_previews")
		expect(keys).toContain("code_block_theme")
		expect(keys).toContain("map_provider")
		expect(keys).toContain("gif_provider")
		expect(keys).toContain("max_image_width")
		expect(keys).toContain("room_view_type")
		expect(keys).toContain("low_bandwidth")
		expect(keys).toContain("server_sent_events")
		expect(keys).toContain("web_push")
	})

	it("each preference has required fields", () => {
		for (const [key, pref] of Object.entries(preferences)) {
			expect(pref.displayName, `${key} missing displayName`).toBeTruthy()
			expect(pref.allowedContexts, `${key} missing allowedContexts`).toBeDefined()
			expect(Array.isArray(pref.allowedContexts), `${key} allowedContexts not array`).toBe(true)
			expect(pref.defaultValue, `${key} missing defaultValue`).toBeDefined()
		}
	})

	it("send_read_receipts defaults to true", () => {
		expect(preferences.send_read_receipts.defaultValue).toBe(true)
	})

	it("show_media_previews defaults to false", () => {
		expect(preferences.show_media_previews.defaultValue).toBe(false)
	})

	it("code_block_theme has allowedValues matching codeBlockStyles", () => {
		expect(preferences.code_block_theme.allowedValues).toBe(codeBlockStyles)
	})

	it("max_image_width has min/max constraints", () => {
		expect(preferences.max_image_width.minValue).toBe(80)
		expect(preferences.max_image_width.maxValue).toBe(1920)
		expect(preferences.max_image_width.defaultValue).toBe(320)
	})

	it("notification_sound_volume has numberType range", () => {
		expect(preferences.notification_sound_volume.numberType).toBe("range")
		expect(preferences.notification_sound_volume.minValue).toBe(0)
		expect(preferences.notification_sound_volume.maxValue).toBe(100)
	})
})

describe("existingPreferenceKeys", () => {
	it("is a Set containing all preference keys", () => {
		expect(existingPreferenceKeys).toBeInstanceOf(Set)
		for (const key of Object.keys(preferences)) {
			expect(existingPreferenceKeys.has(key)).toBe(true)
		}
	})
})

describe("isValidPreferenceKey", () => {
	it("returns true for valid preference keys", () => {
		expect(isValidPreferenceKey("send_read_receipts")).toBe(true)
		expect(isValidPreferenceKey("code_block_theme")).toBe(true)
	})

	it("returns false for non-string values", () => {
		expect(isValidPreferenceKey(42)).toBe(false)
		expect(isValidPreferenceKey(null)).toBe(false)
		expect(isValidPreferenceKey(undefined)).toBe(false)
		expect(isValidPreferenceKey({})).toBe(false)
	})

	it("returns false for unknown string keys", () => {
		expect(isValidPreferenceKey("nonexistent_pref")).toBe(false)
		expect(isValidPreferenceKey("")).toBe(false)
	})
})
