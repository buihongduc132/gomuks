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
import TooltipButton from "./TooltipButton"

beforeEach(() => {
	(globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true
})

describe("TooltipButton", () => {
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

	test("renders button with children", () => {
		act(() => root.render(createElement(TooltipButton, {
			tooltipText: "Click me",
		}, "Label")))
		const btn = container.querySelector("button") as HTMLButtonElement
		expect(btn).toBeTruthy()
		expect(btn.textContent).toContain("Label")
		expect(btn.textContent).toContain("Click me")
	})

	test("defaults tooltip direction to top", () => {
		act(() => root.render(createElement(TooltipButton, {
			tooltipText: "tip",
		}, "X")))
		const tooltip = container.querySelector(".button-tooltip") as HTMLElement
		expect(tooltip.className).toContain("button-tooltip-top")
	})

	test("supports custom tooltip direction", () => {
		act(() => root.render(createElement(TooltipButton, {
			tooltipText: "tip",
			tooltipDirection: "bottom",
		}, "X")))
		const tooltip = container.querySelector(".button-tooltip") as HTMLElement
		expect(tooltip.className).toContain("button-tooltip-bottom")
	})

	test("adds with-tooltip class to button", () => {
		act(() => root.render(createElement(TooltipButton, {
			tooltipText: "tip",
		}, "X")))
		const btn = container.querySelector("button") as HTMLButtonElement
		expect(btn.className).toContain("with-tooltip")
	})

	test("preserves custom className on button", () => {
		act(() => root.render(createElement(TooltipButton, {
			tooltipText: "tip",
			className: "extra",
		}, "X")))
		const btn = container.querySelector("button") as HTMLButtonElement
		expect(btn.className).toContain("with-tooltip")
		expect(btn.className).toContain("extra")
	})

	test("passes tooltipProps to tooltip div", () => {
		act(() => root.render(createElement(TooltipButton, {
			tooltipText: "tip",
			tooltipProps: { id: "my-tooltip", className: "custom-tooltip" },
		}, "X")))
		const tooltip = container.querySelector(".button-tooltip") as HTMLElement
		expect(tooltip.id).toBe("my-tooltip")
		expect(tooltip.className).toContain("custom-tooltip")
	})

	test("forwards button attributes", () => {
		act(() => root.render(createElement(TooltipButton, {
			tooltipText: "tip",
			disabled: true,
			title: "btn-title",
		}, "X")))
		const btn = container.querySelector("button") as HTMLButtonElement
		expect(btn.disabled).toBe(true)
		expect(btn.title).toBe("btn-title")
	})
})
