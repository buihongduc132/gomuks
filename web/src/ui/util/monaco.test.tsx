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

// Mock monaco-editor modules
vi.mock("monaco-editor/esm/vs/basic-languages/css/css.contribution.js", () => ({}))
vi.mock("monaco-editor/esm/vs/editor/edcore.main.js", () => ({}))
vi.mock("monaco-editor/esm/vs/language/css/monaco.contribution.js", () => ({}))
vi.mock("monaco-editor/esm/vs/language/css/css.worker.js?worker", () => ({
	default: class MockWorker {},
}))

const mockModel = {
	getValue: vi.fn(() => ""),
	onDidChangeContent: vi.fn(),
}

type EditorMock = {
	dispose: ReturnType<typeof vi.fn>
	getModel: ReturnType<typeof vi.fn>
	onKeyDown: ReturnType<typeof vi.fn>
	focus: ReturnType<typeof vi.fn>
}

const mockEditor: EditorMock = {
	dispose: vi.fn(),
	getModel: vi.fn(() => mockModel),
	onKeyDown: vi.fn(),
	focus: vi.fn(),
}

vi.mock("monaco-editor/esm/vs/editor/editor.api.js", () => ({
	editor: {
		create: vi.fn(() => mockEditor),
		EndOfLinePreference: { LF: 0 },
	},
	KeyCode: { Escape: 9, KeyS: 49 },
}))

import Monaco from "./monaco"

beforeEach(() => {
	(globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true
	mockEditor.dispose.mockClear()
	mockEditor.onKeyDown.mockClear()
	mockEditor.focus.mockClear()
	mockModel.onDidChangeContent.mockClear()
	// jsdom does not implement matchMedia
	Object.defineProperty(window, "matchMedia", {
		writable: true,
		value: vi.fn().mockImplementation((query: string) => ({
			matches: false,
			media: query,
			onchange: null,
			addListener: vi.fn(),
			removeListener: vi.fn(),
			addEventListener: vi.fn(),
			removeEventListener: vi.fn(),
			dispatchEvent: vi.fn(),
		})),
	})
})

describe("Monaco", () => {
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

	test("renders container div", () => {
		act(() => root.render(createElement(Monaco, {
			initData: "",
			onClose: () => {},
			onSave: () => {},
			contentRef: { current: "" },
		})))
		const div = container.querySelector("div")
		expect(div).toBeTruthy()
	})

	test("creates monaco editor on mount", async () => {
		act(() => root.render(createElement(Monaco, {
			initData: "body { color: red }",
			onClose: () => {},
			onSave: () => {},
			contentRef: { current: "" },
		})))
		const monacoApi = await import("monaco-editor/esm/vs/editor/editor.api.js")
		expect(monacoApi.editor.create).toHaveBeenCalled()
	})

	test("disposes editor on unmount", () => {
		act(() => root.render(createElement(Monaco, {
			initData: "",
			onClose: () => {},
			onSave: () => {},
			contentRef: { current: "" },
		})))
		expect(mockEditor.dispose).not.toHaveBeenCalled()
		act(() => root.unmount())
		expect(mockEditor.dispose).toHaveBeenCalled()
	})

	test("sets up model change listener", () => {
		act(() => root.render(createElement(Monaco, {
			initData: "",
			onClose: () => {},
			onSave: () => {},
			contentRef: { current: "" },
		})))
		expect(mockModel.onDidChangeContent).toHaveBeenCalled()
	})

	test("sets up keydown listener", () => {
		act(() => root.render(createElement(Monaco, {
			initData: "",
			onClose: () => {},
			onSave: () => {},
			contentRef: { current: "" },
		})))
		expect(mockEditor.onKeyDown).toHaveBeenCalled()
	})

	test("focuses editor on mount", () => {
		act(() => root.render(createElement(Monaco, {
			initData: "",
			onClose: () => {},
			onSave: () => {},
			contentRef: { current: "" },
		})))
		expect(mockEditor.focus).toHaveBeenCalled()
	})

	test("key handler calls onClose on Escape", () => {
		const onClose = vi.fn()
		act(() => root.render(createElement(Monaco, {
			initData: "",
			onClose,
			onSave: () => {},
			contentRef: { current: "" },
		})))
		const handler = mockEditor.onKeyDown.mock.calls[0][0] as (evt: { keyCode: number }) => void
		handler({ keyCode: 9 /* Escape */ } as never)
		expect(onClose).toHaveBeenCalled()
	})

	test("key handler calls onSave on Ctrl+S", () => {
		const onSave = vi.fn()
		act(() => root.render(createElement(Monaco, {
			initData: "",
			onClose: () => {},
			onSave,
			contentRef: { current: "" },
		})))
		const handler = mockEditor.onKeyDown.mock.calls[0][0] as
			(evt: { keyCode: number, ctrlKey?: boolean, preventDefault: () => void }) => void
		const evt = { keyCode: 49 /* KeyS */, ctrlKey: true, preventDefault: vi.fn() }
		handler(evt as never)
		expect(onSave).toHaveBeenCalled()
		expect(evt.preventDefault).toHaveBeenCalled()
	})

	test("key handler calls onSave on Meta+S", () => {
		const onSave = vi.fn()
		act(() => root.render(createElement(Monaco, {
			initData: "",
			onClose: () => {},
			onSave,
			contentRef: { current: "" },
		})))
		const handler = mockEditor.onKeyDown.mock.calls[0][0] as
			(evt: { keyCode: number, metaKey?: boolean, preventDefault: () => void }) => void
		handler({ keyCode: 49 /* KeyS */, metaKey: true, preventDefault: vi.fn() } as never)
		expect(onSave).toHaveBeenCalled()
	})

	test("MonacoEnvironment.getWorker returns CSSWorker instance", () => {
		expect(window.MonacoEnvironment).toBeDefined()
		expect(typeof window.MonacoEnvironment!.getWorker).toBe("function")
		const worker = window.MonacoEnvironment!.getWorker!()
		expect(worker).toBeDefined()
	})

	test("model change listener updates contentRef", () => {
		const contentRef = { current: "" }
		act(() => root.render(createElement(Monaco, {
			initData: "",
			onClose: () => {},
			onSave: () => {},
			contentRef,
		})))
		const listener = mockModel.onDidChangeContent.mock.calls[0][0] as () => void
		mockModel.getValue.mockReturnValueOnce("new css")
		listener()
		expect(contentRef.current).toBe("new css")
	})

	test("skips setup when model is null", () => {
		mockEditor.getModel.mockReturnValueOnce(null)
		act(() => root.render(createElement(Monaco, {
			initData: "",
			onClose: () => {},
			onSave: () => {},
			contentRef: { current: "" },
		})))
		expect(mockEditor.onKeyDown).not.toHaveBeenCalled()
	})

	test("uses dark theme when prefers-color-scheme is dark", async () => {
		const monacoApi = await import("monaco-editor/esm/vs/editor/editor.api.js")
		const matchMediaSpy = window.matchMedia as unknown as ReturnType<typeof vi.fn>
		matchMediaSpy.mockImplementationOnce((query: string) => ({
			matches: true,
			media: query,
			onchange: null,
			addListener: vi.fn(),
			removeListener: vi.fn(),
			addEventListener: vi.fn(),
			removeEventListener: vi.fn(),
			dispatchEvent: vi.fn(),
		}))
		act(() => root.render(createElement(Monaco, {
			initData: "",
			onClose: () => {},
			onSave: () => {},
			contentRef: { current: "" },
		})))
		expect(monacoApi.editor.create).toHaveBeenCalledWith(
			expect.anything(),
			expect.objectContaining({ theme: "vs-dark" }),
		)
	})
})
