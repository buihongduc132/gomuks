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
import Toggle from "./Toggle"

beforeEach(() => {
	(globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true
})

describe("Toggle", () => {
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

	test("renders as checkbox input", () => {
		act(() => root.render(createElement(Toggle)))
		const input = container.querySelector("input") as HTMLInputElement
		expect(input).toBeTruthy()
		expect(input.type).toBe("checkbox")
	})

	test("has toggle class", () => {
		act(() => root.render(createElement(Toggle)))
		const input = container.querySelector("input") as HTMLInputElement
		expect(input.className).toBe("toggle")
	})

	test("appends custom className", () => {
		act(() => root.render(createElement(Toggle, { className: "extra" })))
		const input = container.querySelector("input") as HTMLInputElement
		expect(input.className).toBe("toggle extra")
	})

	test("passes disabledColor and enabledColor as CSS variables", () => {
		act(() => root.render(createElement(Toggle, {
			disabledColor: "#ccc",
			enabledColor: "#0f0",
		})))
		const input = container.querySelector("input") as HTMLInputElement
		expect(input.style.getPropertyValue("--disabled-color")).toBe("#ccc")
		expect(input.style.getPropertyValue("--enabled-color")).toBe("#0f0")
	})

	test("forwards onChange handler", () => {
		const onChange = vi.fn()
		act(() => root.render(createElement(Toggle, { onChange })))
		const input = container.querySelector("input") as HTMLInputElement
		act(() => input.click())
		expect(onChange).toHaveBeenCalled()
	})
})
