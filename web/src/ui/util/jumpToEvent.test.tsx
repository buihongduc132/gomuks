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

// Mock heavy deps
vi.mock("../modal", () => ({
	modals: {
		eventContext: vi.fn((_roomCtx: unknown, evtID: string) => ({ type: "eventContext", evtID })),
	},
}))
vi.mock("../roomview/roomcontext.ts", () => ({
	RoomContextData: class MockRoomContextData {
		store = { timeline: [] }
		scrolledToBottom = true
	},
}))

// Must declare global for TypeScript
declare global {
	interface Window {
		openNestableModal: (modal: unknown) => void
	}
}

import { jumpToEvent, jumpToVisibleEvent, jumpToEventInView } from "./jumpToEvent"

describe("jumpToVisibleEvent", () => {
	beforeEach(() => {
		document.body.innerHTML = ""
	})

	test("returns false when evtID is empty", () => {
		expect(jumpToVisibleEvent("")).toBe(false)
	})

	test("returns false when element not found", () => {
		expect(jumpToVisibleEvent("$event123")).toBe(false)
	})

	test("returns true and scrolls element into view when found", () => {
		const el = document.createElement("div")
		el.setAttribute("data-event-id", "$event123")
		document.body.appendChild(el)
		el.scrollIntoView = vi.fn()
		el.classList.add = vi.fn()

		expect(jumpToVisibleEvent("$event123")).toBe(true)
		expect(el.scrollIntoView).toHaveBeenCalledWith({ block: "center" })
		expect(el.classList.add).toHaveBeenCalledWith("jump-highlight")
	})

	test("uses parent element for querySelector when provided", () => {
		const parent = document.createElement("div")
		const el = document.createElement("div")
		el.setAttribute("data-event-id", "$evt")
		parent.appendChild(el)
		el.scrollIntoView = vi.fn()
		el.classList.add = vi.fn()

		expect(jumpToVisibleEvent("$evt", parent)).toBe(true)
		expect(el.scrollIntoView).toHaveBeenCalled()
	})

	test("updates scrolledToBottom when roomCtx provided", () => {
		const parent = document.createElement("div")
		Object.defineProperty(parent, "scrollHeight", { value: 500 })
		Object.defineProperty(parent, "scrollTop", { value: 100 })
		Object.defineProperty(parent, "clientHeight", { value: 400 })
		const el = document.createElement("div")
		el.setAttribute("data-event-id", "$evt")
		parent.appendChild(el)
		el.scrollIntoView = vi.fn()
		el.classList.add = vi.fn()

		const roomCtx = { scrolledToBottom: false } as any
		jumpToVisibleEvent("$evt", parent, roomCtx)
		// scrollHeight(500) - scrollTop(100) - clientHeight(400) = 0, which is < 5
		expect(roomCtx.scrolledToBottom).toBe(true)
	})

	test("adds fadeout classes after timeout", () => {
		vi.useFakeTimers()
		const el = document.createElement("div")
		el.setAttribute("data-event-id", "$evt")
		document.body.appendChild(el)
		el.scrollIntoView = vi.fn()

		jumpToVisibleEvent("$evt")
		expect(el.classList.contains("jump-highlight")).toBe(true)

		// Advance past 3000ms to trigger fadeout
		vi.advanceTimersByTime(3000)
		expect(el.classList.contains("jump-highlight-fadeout")).toBe(true)
		expect(el.classList.contains("jump-highlight")).toBe(false)

		// Advance past 1500ms to remove fadeout
		vi.advanceTimersByTime(1500)
		expect(el.classList.contains("jump-highlight-fadeout")).toBe(false)

		vi.useRealTimers()
	})
})

describe("jumpToEvent", () => {
	beforeEach(() => {
		document.body.innerHTML = ""
		window.openNestableModal = vi.fn()
		vi.useFakeTimers()
	})

	afterEach(() => {
		vi.useRealTimers()
	})

	test("jumps to visible event without opening modal", () => {
		const el = document.createElement("div")
		el.setAttribute("data-event-id", "$evt1")
		document.body.appendChild(el)
		el.scrollIntoView = vi.fn()
		el.classList.add = vi.fn()

		const roomCtx = { store: { timeline: [{ event_rowid: 1 }] }, scrolledToBottom: true } as any
		jumpToEvent(roomCtx, "$evt1")
		expect(window.openNestableModal).not.toHaveBeenCalled()
	})

	test("opens modal when event not visible and timeline empty (no retry)", () => {
		const roomCtx = { store: { timeline: [] }, scrolledToBottom: true } as any
		jumpToEvent(roomCtx, "$evt2", false)
		expect(window.openNestableModal).toHaveBeenCalled()
	})

	test("retries with setTimeout when timeline empty and allowRetry=true", () => {
		const roomCtx = { store: { timeline: [] }, scrolledToBottom: true } as any
		jumpToEvent(roomCtx, "$evt3", true)
		// Should not open modal yet
		expect(window.openNestableModal).not.toHaveBeenCalled()
		// Advance timer
		vi.advanceTimersByTime(600)
		// After retry, timeline still empty, so modal opens
		expect(window.openNestableModal).toHaveBeenCalled()
	})
})

describe("jumpToEventInView", () => {
	beforeEach(() => {
		document.body.innerHTML = ""
		window.openNestableModal = vi.fn()
	})

	test("uses modal when parent is null", () => {
		const roomCtx = { store: { timeline: [] }, scrolledToBottom: true } as any
		jumpToEventInView(roomCtx, "$evt4", null)
		expect(window.openNestableModal).toHaveBeenCalled()
	})

	test("uses modal when element not found in parent", () => {
		const parent = document.createElement("div")
		const roomCtx = { store: { timeline: [] }, scrolledToBottom: true } as any
		jumpToEventInView(roomCtx, "$evt5", parent)
		expect(window.openNestableModal).toHaveBeenCalled()
	})

	test("does not open modal when element found", () => {
		const parent = document.createElement("div")
		const el = document.createElement("div")
		el.setAttribute("data-event-id", "$evt6")
		parent.appendChild(el)
		el.scrollIntoView = vi.fn()
		el.classList.add = vi.fn()

		const roomCtx = { store: { timeline: [] }, scrolledToBottom: true } as any
		jumpToEventInView(roomCtx, "$evt6", parent)
		expect(window.openNestableModal).not.toHaveBeenCalled()
	})
})
