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
import { act } from "react"
import { createRoot, type Root } from "react-dom/client"
import { createElement } from "react"
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest"
import ResizeHandle from "./ResizeHandle"

beforeEach(() => {
	(globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true
})

describe("ResizeHandle", () => {
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

	test("renders outer and inner divs", () => {
		const setWidth = vi.fn()
		act(() => root.render(createElement(ResizeHandle, {
			width: 200, minWidth: 100, maxWidth: 400, setWidth,
		})))
		expect(container.querySelector(".resize-handle-outer")).toBeTruthy()
		expect(container.querySelector(".resize-handle-inner")).toBeTruthy()
	})

	test("applies custom className and style", () => {
		const setWidth = vi.fn()
		act(() => root.render(createElement(ResizeHandle, {
			width: 200, minWidth: 100, maxWidth: 400, setWidth,
			className: "custom", style: { background: "red" },
		})))
		const outer = container.querySelector(".resize-handle-outer") as HTMLElement
		expect(outer.className).toContain("custom")
		expect(outer.style.background).toBe("red")
	})

	test("mousedown + mousemove updates width", () => {
		const setWidth = vi.fn()
		act(() => root.render(createElement(ResizeHandle, {
			width: 200, minWidth: 100, maxWidth: 400, setWidth,
		})))
		const inner = container.querySelector(".resize-handle-inner") as HTMLElement
		// mousedown at x=100
		act(() => inner.dispatchEvent(new MouseEvent("mousedown", { clientX: 100, bubbles: true })))
		// mousemove to x=150 → delta=50, new width=250
		document.dispatchEvent(new MouseEvent("mousemove", { clientX: 150 }))
		expect(setWidth).toHaveBeenCalledWith(250)
	})

	test("mousedown + mousemove clamps to maxWidth", () => {
		const setWidth = vi.fn()
		act(() => root.render(createElement(ResizeHandle, {
			width: 200, minWidth: 100, maxWidth: 400, setWidth,
		})))
		const inner = container.querySelector(".resize-handle-inner") as HTMLElement
		act(() => inner.dispatchEvent(new MouseEvent("mousedown", { clientX: 100, bubbles: true })))
		document.dispatchEvent(new MouseEvent("mousemove", { clientX: 500 }))
		expect(setWidth).toHaveBeenCalledWith(400)
	})

	test("mousedown + mousemove clamps to minWidth", () => {
		const setWidth = vi.fn()
		act(() => root.render(createElement(ResizeHandle, {
			width: 200, minWidth: 100, maxWidth: 400, setWidth,
		})))
		const inner = container.querySelector(".resize-handle-inner") as HTMLElement
		act(() => inner.dispatchEvent(new MouseEvent("mousedown", { clientX: 100, bubbles: true })))
		document.dispatchEvent(new MouseEvent("mousemove", { clientX: -100 }))
		expect(setWidth).toHaveBeenCalledWith(100)
	})

	test("inverted reverses delta", () => {
		const setWidth = vi.fn()
		act(() => root.render(createElement(ResizeHandle, {
			width: 200, minWidth: 100, maxWidth: 400, setWidth, inverted: true,
		})))
		const inner = container.querySelector(".resize-handle-inner") as HTMLElement
		act(() => inner.dispatchEvent(new MouseEvent("mousedown", { clientX: 100, bubbles: true })))
		// delta = 50, inverted → -50, new width = 150
		document.dispatchEvent(new MouseEvent("mousemove", { clientX: 150 }))
		expect(setWidth).toHaveBeenCalledWith(150)
	})

	test("mouseup removes event listeners", () => {
		const setWidth = vi.fn()
		act(() => root.render(createElement(ResizeHandle, {
			width: 200, minWidth: 100, maxWidth: 400, setWidth,
		})))
		const inner = container.querySelector(".resize-handle-inner") as HTMLElement
		act(() => inner.dispatchEvent(new MouseEvent("mousedown", { clientX: 100, bubbles: true })))
		document.dispatchEvent(new MouseEvent("mouseup"))
		// After mouseup, further mousemove should not call setWidth
		setWidth.mockClear()
		document.dispatchEvent(new MouseEvent("mousemove", { clientX: 300 }))
		expect(setWidth).not.toHaveBeenCalled()
	})
})
