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
import JSONView from "./JSONView"

beforeEach(() => {
	(globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true
})

function renderToText(element: React.ReactElement): string {
	const container = document.createElement("div")
	document.body.appendChild(container)
	const root = createRoot(container)
	act(() => root.render(element))
	const text = container.textContent ?? ""
	act(() => root.unmount())
	container.remove()
	return text
}

describe("JSONView", () => {
	test("renders string data", () => {
		const text = renderToText(createElement(JSONView, { data: "hello" }))
		expect(text).toContain("hello")
	})

	test("renders number data", () => {
		const text = renderToText(createElement(JSONView, { data: 42 }))
		expect(text).toContain("42")
	})

	test("renders boolean true", () => {
		const text = renderToText(createElement(JSONView, { data: true }))
		expect(text).toContain("true")
	})

	test("renders boolean false", () => {
		const text = renderToText(createElement(JSONView, { data: false }))
		expect(text).toContain("false")
	})

	test("renders null", () => {
		const text = renderToText(createElement(JSONView, { data: null }))
		expect(text).toContain("null")
	})

	test("renders undefined", () => {
		const text = renderToText(createElement(JSONView, { data: undefined }))
		expect(text).toContain("undefined")
	})

	test("renders empty array as nothing", () => {
		const text = renderToText(createElement(JSONView, { data: [] }))
		expect(text).not.toContain("…")
	})

	test("renders empty object as nothing", () => {
		const text = renderToText(createElement(JSONView, { data: {} }))
		expect(text).not.toContain("…")
	})

	test("renders array with items", () => {
		const text = renderToText(createElement(JSONView, { data: [1, "two", true] }))
		expect(text).toContain("1")
		expect(text).toContain("two")
		expect(text).toContain("true")
	})

	test("renders object with entries", () => {
		const text = renderToText(createElement(JSONView, { data: { foo: "bar", baz: 42 } }))
		expect(text).toContain("foo")
		expect(text).toContain("bar")
		expect(text).toContain("baz")
		expect(text).toContain("42")
	})

	test("renders nested object", () => {
		const text = renderToText(createElement(JSONView, { data: { a: { b: 1 } } }))
		expect(text).toContain("a")
		expect(text).toContain("b")
		expect(text).toContain("1")
	})

	test("renders collapsed array as ellipsis", () => {
		// Click collapse button to collapse, then verify (nested array gets collapse button)
		const container = document.createElement("div")
		document.body.appendChild(container)
		const root = createRoot(container)
		act(() => root.render(createElement(JSONView, { data: { arr: [1, 2, 3] } })))
		const collapseBtn = container.querySelector(".button")
		expect(collapseBtn).toBeTruthy()
		// Initially expanded
		expect(container.textContent).toContain("1")
		// Click to collapse
		act(() => collapseBtn!.dispatchEvent(new MouseEvent("click", { bubbles: true })))
		expect(container.textContent).toContain("…")
		act(() => root.unmount())
		container.remove()
	})

	test("renders undefined values in object as null", () => {
		const text = renderToText(createElement(JSONView, { data: { a: undefined, b: 1 } }))
		expect(text).toContain("b")
		expect(text).toContain("1")
	})
})
