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
	Preference,
	PreferenceContext,
	anyContext,
	anyGlobalContext,
	deviceSpecific,
	globalDeviceSpecific,
	preferenceContextToInt,
	roomSpecific,
} from "./types.ts"

describe("preferenceContextToInt", () => {
	it("maps each context to its numeric value", () => {
		expect(preferenceContextToInt(PreferenceContext.Config)).toBe(0)
		expect(preferenceContextToInt(PreferenceContext.Account)).toBe(1)
		expect(preferenceContextToInt(PreferenceContext.Device)).toBe(2)
		expect(preferenceContextToInt(PreferenceContext.RoomAccount)).toBe(3)
		expect(preferenceContextToInt(PreferenceContext.RoomDevice)).toBe(4)
	})
})

describe("context arrays", () => {
	it("anyContext contains all five contexts", () => {
		expect(anyContext).toHaveLength(5)
		expect(anyContext).toContain(PreferenceContext.Config)
		expect(anyContext).toContain(PreferenceContext.Account)
		expect(anyContext).toContain(PreferenceContext.Device)
		expect(anyContext).toContain(PreferenceContext.RoomAccount)
		expect(anyContext).toContain(PreferenceContext.RoomDevice)
	})

	it("anyGlobalContext excludes room-specific contexts", () => {
		expect(anyGlobalContext).toHaveLength(3)
		expect(anyGlobalContext).not.toContain(PreferenceContext.RoomAccount)
		expect(anyGlobalContext).not.toContain(PreferenceContext.RoomDevice)
	})

	it("deviceSpecific only has device contexts", () => {
		expect(deviceSpecific).toEqual([
			PreferenceContext.RoomDevice,
			PreferenceContext.Device,
		])
	})

	it("globalDeviceSpecific only has global device contexts", () => {
		expect(globalDeviceSpecific).toEqual([
			PreferenceContext.Device,
			PreferenceContext.Config,
		])
	})

	it("roomSpecific only has room contexts", () => {
		expect(roomSpecific).toEqual([
			PreferenceContext.RoomAccount,
			PreferenceContext.RoomDevice,
		])
	})
})

describe("Preference class", () => {
	it("sets all fields from constructor", () => {
		const pref = new Preference<boolean>({
			displayName: "Test pref",
			allowedContexts: anyGlobalContext,
			defaultValue: true,
			description: "A test preference",
		})
		expect(pref.displayName).toBe("Test pref")
		expect(pref.allowedContexts).toBe(anyGlobalContext)
		expect(pref.defaultValue).toBe(true)
		expect(pref.description).toBe("A test preference")
		expect(pref.hidden).toBe(false)
		expect(pref.allowedValues).toBeUndefined()
		expect(pref.valueLabels).toBeUndefined()
		expect(pref.minValue).toBeUndefined()
		expect(pref.maxValue).toBeUndefined()
		expect(pref.numberType).toBeUndefined()
	})

	it("defaults description to empty string and hidden to false", () => {
		const pref = new Preference<number>({
			displayName: "No description",
			allowedContexts: anyContext,
			defaultValue: 0,
			// description omitted
		} as never)
		expect(pref.description).toBe("")
		expect(pref.hidden).toBe(false)
	})

	it("sets hidden to true when specified", () => {
		const pref = new Preference<string>({
			displayName: "Hidden pref",
			allowedContexts: anyContext,
			defaultValue: "x",
			hidden: true,
		})
		expect(pref.hidden).toBe(true)
	})

	it("stores min/max/numberType for numeric prefs", () => {
		const pref = new Preference<number>({
			displayName: "Range pref",
			allowedContexts: anyContext,
			defaultValue: 50,
			minValue: 0,
			maxValue: 100,
			numberType: "range",
		})
		expect(pref.minValue).toBe(0)
		expect(pref.maxValue).toBe(100)
		expect(pref.numberType).toBe("range")
	})

	it("stores allowedValues and valueLabels", () => {
		const pref = new Preference<string>({
			displayName: "Enum pref",
			allowedContexts: anyContext,
			defaultValue: "a",
			allowedValues: ["a", "b"] as const,
			valueLabels: ["Option A", "Option B"] as const,
		})
		expect(pref.allowedValues).toEqual(["a", "b"])
		expect(pref.valueLabels).toEqual(["Option A", "Option B"])
	})
})
