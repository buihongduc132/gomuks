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
import ErrorBoundary from "./ErrorBoundary"

beforeEach(() => {
	(globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true
})

describe("ErrorBoundary", () => {
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

	test("renders children when no error", () => {
		act(() => root.render(createElement(ErrorBoundary, null, "hello")))
		expect(container.textContent).toBe("hello")
	})

	test("renders error message when child throws", () => {
		function Broken(): never {
			throw new Error("kaboom")
		}

		// React logs the error; silence it for this test
		const consoleError = console.error
		console.error = () => {}
		try {
			act(() => root.render(createElement(ErrorBoundary, null, createElement(Broken))))
		} finally {
			console.error = consoleError
		}
		expect(container.textContent).toContain("Failed to render component: kaboom")
	})

	test("strips Error prefix from message", () => {
		const state = ErrorBoundary.getDerivedStateFromError(new Error("prefix test"))
		expect(state).toEqual({ error: "prefix test" })
	})

	test("uses custom thing name in error message", () => {
		const consoleError = console.error
		console.error = () => {}
		function Broken(): never {
			throw new Error("xyz")
		}
		try {
			act(() => root.render(createElement(
				ErrorBoundary,
				{ thing: "timeline" },
				createElement(Broken),
			)))
		} finally {
			console.error = consoleError
		}
		expect(container.textContent).toContain("Failed to render timeline: xyz")
	})

	test("wraps error in div with wrapperClassName", () => {
		const consoleError = console.error
		console.error = () => {}
		function Broken(): never {
			throw new Error("wrapped")
		}
		try {
			act(() => root.render(createElement(
				ErrorBoundary,
				{ wrapperClassName: "err-wrap" },
				createElement(Broken),
			)))
		} finally {
			console.error = consoleError
		}
		const wrap = container.querySelector("div.err-wrap")
		expect(wrap).toBeTruthy()
		expect(wrap!.textContent).toContain("Failed to render component: wrapped")
	})
})
