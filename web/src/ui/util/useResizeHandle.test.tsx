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
import { afterEach, beforeEach, describe, expect, test } from "vitest"
import { useResizeHandle } from "./useResizeHandle"

beforeEach(() => {
	(globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true
	localStorage.clear()
})

describe("useResizeHandle", () => {
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

	test("returns default size when nothing saved", () => {
		let width: number | undefined
		function TestComponent() {
			const [w] = useResizeHandle(200, 100, 400, "test-key", {})
			width = w
			return null
		}
		act(() => root.render(createElement(TestComponent)))
		expect(width).toBe(200)
	})

	test("restores saved width from localStorage", () => {
		localStorage.setItem("test-key", "350")
		let width: number | undefined
		function TestComponent() {
			const [w] = useResizeHandle(200, 100, 400, "test-key", {})
			width = w
			return null
		}
		act(() => root.render(createElement(TestComponent)))
		expect(width).toBe(350)
	})

	test("renders ResizeHandle element", () => {
		let handle: React.ReactNode
		function TestComponent() {
			const [, h] = useResizeHandle(200, 100, 400, "test-key", {})
			handle = h
			return createElement("div", null, h)
		}
		act(() => root.render(createElement(TestComponent)))
		expect(container.querySelector(".resize-handle-outer")).toBeTruthy()
	})

	test("saves width to localStorage on change", () => {
		let setWidthFn: ((w: number) => void) | undefined
		function TestComponent() {
			const [w, handle] = useResizeHandle(200, 100, 400, "test-key", {})
			// Extract setWidth from the handle props
			if (handle && typeof handle === "object" && "props" in handle) {
				setWidthFn = (handle as any).props.setWidth
			}
			return createElement("div", null, handle)
		}
		act(() => root.render(createElement(TestComponent)))
		// Trigger a width change via the ResizeHandle's setWidth
		if (setWidthFn) {
			act(() => setWidthFn!(250))
		}
		expect(localStorage.getItem("test-key")).toBe("250")
	})
})
