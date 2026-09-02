import { describe, it, expect, vi, beforeEach } from "vitest"

// Hook state shared with the mocked useSyncExternalStore so tests can
// register a persistent listener (subscribe without unsubscribing) and
// manually trigger the unsubscribe branch for branch coverage.
const hookState: { notified: number, unsubscribe?: () => void } = { notified: 0 }

vi.mock("react", async (importOriginal) => {
	const actual = await importOriginal<typeof import("react")>()
	return {
		...actual,
		useSyncExternalStore: (
			subscribe: (cb: () => void) => () => void,
			getSnapshot: () => unknown,
		) => {
			hookState.unsubscribe = subscribe(() => { hookState.notified++ })
			return getSnapshot()
		},
	}
})

const testTabs = [
	{ id: "tab-1", displayname: "Tab 1", type: "embedded" as const, disable_notifications: false, unread: 5, exited: false },
	{ id: "tab-2", displayname: "Tab 2", type: "remote" as const, disable_notifications: false, unread: 3, exited: false },
]

// Single persistent desktop API mock: the module caches `api` at import time,
// so the object identity must stay stable across the primary describe's tests.
const mockDesktop = {
	isDesktop: true as const,
	getTabID: vi.fn<(tab?: string) => string>().mockReturnValue("tab-1"),
	isEmbedded: vi.fn().mockReturnValue(true),
	setNotificationCount: vi.fn(),
	switchTab: vi.fn(),
	updateTab: vi.fn().mockResolvedValue(undefined),
	deleteTab: vi.fn().mockResolvedValue(undefined),
	restartBackend: vi.fn(),
	getDisableNotifications: vi.fn().mockReturnValue(false),
	subscribeToTabs: vi.fn(),
	quitApp: vi.fn(),
}

describe("tabs (desktop API — primary instance)", () => {
	let desktopSubscriber: (tabs: typeof testTabs) => void

	beforeEach(() => {
		vi.clearAllMocks()
		hookState.notified = 0
		hookState.unsubscribe = undefined
		mockDesktop.getTabID.mockReturnValue("tab-1")
		;(window as any).gomuksDesktop = mockDesktop
		delete (window as any).gomuksAndroid
	})

	it("hasTabs returns true when desktop API present", async () => {
		const { hasTabs } = await import("./tabs")
		// First import executes the module: subscribeToTabs is captured here.
		// (Module is cached for the rest of this describe; calls are cleared per-test.)
		desktopSubscriber = mockDesktop.subscribeToTabs.mock.calls[0][0]
		expect(hasTabs()).toBe(true)
	})

	it("getTabsAPI returns desktop API", async () => {
		const { getTabsAPI } = await import("./tabs")
		expect(getTabsAPI()).toBe(mockDesktop)
	})

	it("subscribes to tab updates via desktop API at import", () => {
		// The import in the first test registered this callback
		expect(typeof desktopSubscriber).toBe("function")
	})

	it("onTabUpdate from desktop notifies subscribers and updates cache", async () => {
		const tabsMod = await import("./tabs")
		// Register a persistent listener via the hook
		tabsMod.useTabs()
		expect(hookState.notified).toBe(0)
		desktopSubscriber(testTabs)
		expect(hookState.notified).toBe(1)
		// second update notifies again
		desktopSubscriber(testTabs)
		expect(hookState.notified).toBe(2)
		// Unsubscribing stops notifications (filter-removal branch)
		hookState.unsubscribe!()
		desktopSubscriber(testTabs)
		expect(hookState.notified).toBe(2)
	})

	it("useTabs returns tabs, currentTabID, totalUnreads and API methods", async () => {
		const tabsMod = await import("./tabs")
		desktopSubscriber(testTabs)
		const value = tabsMod.useTabs()
		hookState.unsubscribe!()
		expect(value.hasTabs).toBe(true)
		expect(value.tabs).toEqual(testTabs)
		expect(value.currentTabID).toBe("tab-1")
		// unreads from tab-2 only (current tab excluded)
		expect(value.totalUnreads).toBe(3)
		expect(value.switchTab).toBe(mockDesktop.switchTab)
		expect(value.updateTab).toBe(mockDesktop.updateTab)
		expect(value.deleteTab).toBe(mockDesktop.deleteTab)
	})

	it("useTabs falls back to empty string when getTabID undefined", async () => {
		const tabsMod = await import("./tabs")
		mockDesktop.getTabID.mockReturnValue(undefined)
		desktopSubscriber(testTabs)
		const value = tabsMod.useTabs()
		hookState.unsubscribe!()
		expect(value.currentTabID).toBe("")
		expect(value.totalUnreads).toBe(8)
	})
})

describe("tabs (android API)", () => {
	let mockAndroid: any

	beforeEach(() => {
		vi.clearAllMocks()
		vi.resetModules()
		hookState.notified = 0
		hookState.unsubscribe = undefined
		delete (window as any).gomuksDesktop
		mockAndroid = {
			isAndroid: true,
			getTabID: vi.fn().mockReturnValue("android-tab-1"),
			isEmbedded: vi.fn().mockReturnValue(false),
			setNotificationCount: vi.fn(),
			switchTab: vi.fn(),
			updateTab: vi.fn().mockResolvedValue(undefined),
			deleteTab: vi.fn().mockResolvedValue(undefined),
			restartBackend: vi.fn(),
		}
		;(window as any).gomuksAndroid = mockAndroid
	})

	it("getTabsAPI returns android API", async () => {
		const { getTabsAPI, hasTabs } = await import("./tabs")
		expect(hasTabs()).toBe(true)
		expect(getTabsAPI()).toBe(mockAndroid)
	})

	it("registers GomuksAndroidTabUpdate event listener at import", async () => {
		const addSpy = vi.spyOn(window, "addEventListener")
		await import("./tabs")
		expect(addSpy).toHaveBeenCalledWith("GomuksAndroidTabUpdate", expect.any(Function))
	})

	it("GomuksAndroidTabUpdate event updates tabs cache and notifies", async () => {
		const tabsMod = await import("./tabs")
		tabsMod.useTabs()
		expect(hookState.notified).toBe(0)
		window.dispatchEvent(new CustomEvent("GomuksAndroidTabUpdate", {
			detail: JSON.stringify([{ id: "a", displayname: "A", type: "embedded", disable_notifications: false, unread: 1, exited: false }]),
		}))
		expect(hookState.notified).toBe(1)
	})

	it("useTabs works with android API", async () => {
		const tabsMod = await import("./tabs")
		window.dispatchEvent(new CustomEvent("GomuksAndroidTabUpdate", {
			detail: JSON.stringify(testTabs),
		}))
		const value = tabsMod.useTabs()
		hookState.unsubscribe!()
		expect(value.hasTabs).toBe(true)
		expect(value.currentTabID).toBe("android-tab-1")
		expect(value.totalUnreads).toBe(8)
		expect(value.switchTab).toBe(mockAndroid.switchTab)
	})
})

describe("tabs (no API)", () => {
	beforeEach(() => {
		vi.clearAllMocks()
		vi.resetModules()
		hookState.notified = 0
		hookState.unsubscribe = undefined
		delete (window as any).gomuksDesktop
		delete (window as any).gomuksAndroid
	})

	it("hasTabs returns false and getTabsAPI returns null", async () => {
		const { hasTabs, getTabsAPI } = await import("./tabs")
		expect(hasTabs()).toBe(false)
		expect(getTabsAPI()).toBeNull()
	})

	it("useTabs returns noTabs constant with noop functions", async () => {
		const { useTabs } = await import("./tabs")
		const value = useTabs()
		expect(value.hasTabs).toBe(false)
		expect(value.tabs).toEqual([])
		expect(value.currentTabID).toBe("")
		expect(value.totalUnreads).toBe(0)
		expect(typeof value.switchTab).toBe("function")
		expect(typeof value.updateTab).toBe("function")
		expect(typeof value.deleteTab).toBe("function")
		// noop functions resolve without error
		await expect(value.updateTab({ id: "x", displayname: "x", type: "embedded", disable_notifications: false })).resolves.toBeUndefined()
		await expect(value.deleteTab("x")).resolves.toBeUndefined()
	})

	it("subscribeTabs returns noop unsubscribe when no API", async () => {
		const { useTabs } = await import("./tabs")
		expect(() => useTabs()).not.toThrow()
		// the noop unsubscribe is returned to the hook; calling it is a no-op
		expect(() => hookState.unsubscribe?.()).not.toThrow()
	})
})
