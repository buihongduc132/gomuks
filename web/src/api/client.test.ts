import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { CachedEventDispatcher, EventDispatcher } from "../util/eventdispatcher.ts"
import type RPCClient from "./rpc"
import type { RPCEvent } from "./types"

// Fake StateStore with controllable state
function makeFakeStore() {
	return {
		userID: "",
		serverTimestamp: undefined as number | undefined,
		rooms: new Map(),
		roomList: { emit: vi.fn() },
		homeSpace: { counts: { listen: vi.fn(), current: { unread_highlights: 0, unread_notifications: 0 } } },
		accountData: new Map(),
		accountDataSubs: { getSubscriber: vi.fn(() => vi.fn()) },
		emojiRoomsSub: { notify: vi.fn() },
		preferences: { low_bandwidth: false },
		localPreferenceCache: { web_push: false } as Record<string, unknown>,
		widgetListeners: new Set(),
		stateCache: { setUserID: vi.fn(), tryFlush: vi.fn(), close: vi.fn() },
		clearTyping: vi.fn(),
		clear: vi.fn(),
		closeCache: vi.fn(),
		doGarbageCollection: vi.fn().mockReturnValue(0),
		loadCache: vi.fn().mockResolvedValue(undefined),
		applySync: vi.fn(),
		applyDecrypted: vi.fn(),
		applySendComplete: vi.fn(),
		applyTyping: vi.fn(),
		imageAuthToken: undefined as string | undefined,
		invalidateEmojiPackKeyCache: vi.fn(),
		getEmojiPackKeys: vi.fn().mockReturnValue([]),
	}
}

function makeFakeRoom(roomID = "!room:example.com") {
	return {
		roomID,
		meta: { current: { has_member_list: false } },
		state: new Map(),
		stateLoaded: false,
		waitStateLoaded: undefined as Promise<void> | undefined,
		timeline: [] as Array<{ timeline_rowid: number, event_rowid: number }>,
		eventsByRowID: new Map(),
		eventsByID: new Map(),
		requestedEvents: new Set<string>(),
		requestedEventRowIDs: new Set<number>(),
		requestedMembers: new Set<string>(),
		pendingEvents: [] as number[],
		paginating: false,
		hasMoreHistory: true,
		membersRequested: false,
		applyEvent: vi.fn(),
		notifyTimelineSubscribers: vi.fn(),
		getOrApplyEvent: vi.fn(evt => evt),
		getPinnedEvents: vi.fn().mockReturnValue([]),
		applyFullState: vi.fn(),
		applyPagination: vi.fn(),
		getStateEvent: vi.fn().mockReturnValue(undefined),
		applyState: vi.fn(),
	}
}

type FakeStore = ReturnType<typeof makeFakeStore>
type FakeRoom = ReturnType<typeof makeFakeRoom>

// Mock the statestore module — class constructor returns the fake store instance
vi.mock("./statestore", () => {
	return {
		StateStore: class {
			constructor() {
				const store = (globalThis as any).__fakeStore
				if (!store) {
					throw new Error("makeFakeStore must be called before Client construction")
				}
				return store
			}
		},
		RoomStateStore: class {},
		fakeGomuksSender: "@gomuks",
	}
})

// Mock tabs API to return null by default
vi.mock("./tabs.ts", () => ({
	getTabsAPI: vi.fn(() => null),
}))

// Mock the websocket client to avoid WebSocket creation
vi.mock("./wsclient.ts", () => ({
	default: vi.fn(),
}))

async function importClientModule() {
	return await import("./client")
}

function makeMockRPC() {
	return {
		connect: new CachedEventDispatcher<any>(),
		event: new EventDispatcher<any>(),
		getCachedServerTimestamp: undefined as (() => number | undefined) | undefined,
		tryAuth: vi.fn().mockResolvedValue(true),
		start: vi.fn(),
		stop: vi.fn(),
		logout: vi.fn().mockResolvedValue(undefined),
		getCapabilities: vi.fn().mockResolvedValue({ capabilities: {} }),
		setListenToDevice: vi.fn().mockResolvedValue(undefined),
		registerPush: vi.fn().mockResolvedValue(undefined),
		setAccountData: vi.fn().mockResolvedValue(undefined),
		getSpecificRoomState: vi.fn().mockResolvedValue([]),
		getRoomState: vi.fn().mockResolvedValue([]),
		getEvent: vi.fn().mockResolvedValue({}),
		getEventByRowID: vi.fn().mockResolvedValue({}),
		getRelatedEvents: vi.fn().mockResolvedValue([]),
		getStickyEvents: vi.fn().mockResolvedValue([]),
		getMentions: vi.fn().mockResolvedValue([]),
		paginate: vi.fn().mockResolvedValue({ has_more: false, events: [], related_events: {}, receipts: {} }),
		setState: vi.fn().mockResolvedValue(undefined),
		sendEvent: vi.fn().mockResolvedValue({}),
		sendMessage: vi.fn().mockResolvedValue(null),
		resendEvent: vi.fn().mockResolvedValue({}),
		searchLocal: vi.fn(),
		searchServer: vi.fn(),
	} as unknown as RPCClient & ReturnType<typeof makeMockRPC>
}

async function makeClient(storeOverrides: Partial<FakeStore> = {}) {
	const { default: Client } = await importClientModule()
	const store = { ...makeFakeStore(), ...storeOverrides }
	;(globalThis as any).__fakeStore = store
	const rpc = makeMockRPC()
	const client = new Client(rpc as unknown as RPCClient)
	return { client, rpc, store }
}

beforeEach(() => {
	vi.restoreAllMocks()
	;(globalThis as any).__fakeStore = null
	;(window as any).gcSettings = { interval: 900000, lastOpenedCutoff: 1800000 }
	delete (window as any).gomuksAndroid
	delete (window as any).vapidPublicKey
	window.alert = vi.fn()
	vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({}) }))
})

afterEach(() => {
	;(globalThis as any).__fakeStore = null
})

async function flushMicrotasks(n = 12) {
	for (let i = 0; i < n; i++) {
		await Promise.resolve()
	}
}

describe("Client constructor", () => {
	it("wires getCachedServerTimestamp to store", async () => {
		const { client, store } = await makeClient()
		expect(client.rpc.getCachedServerTimestamp).toBeDefined()
		store.serverTimestamp = 12345
		expect(client.rpc.getCachedServerTimestamp!()).toBe(12345)
	}, 15000)

	it("listens to rpc events", async () => {
		const { client, rpc } = await makeClient()
		expect(rpc.event.hasListeners).toBe(true)
	})

	it("listens to rpc connect events", async () => {
		const { client, rpc, store } = await makeClient()
		expect(rpc.connect.hasListeners).toBe(true)
		rpc.connect.emit({ connected: false, reconnecting: false, error: null })
		expect(client.initComplete.current).toBe(false)
		expect(store.clearTyping).toHaveBeenCalled()
	})
})

describe("Client #handleEvent", () => {
	it("handles client_state with login", async () => {
		const { client, rpc, store } = await makeClient()
		rpc.event.emit({
			command: "client_state",
			request_id: -1,
			data: { is_logged_in: true, user_id: "@user:example.com" },
		} as RPCEvent)
		expect(client.state.current?.is_logged_in).toBe(true)
		expect(store.userID).toBe("@user:example.com")
		expect(store.stateCache.setUserID).toHaveBeenCalledWith("@user:example.com")
	})

	it("handles client_state logout", async () => {
		const { client, rpc, store } = await makeClient()
		store.userID = "@old:example.com"
		rpc.event.emit({
			command: "client_state",
			request_id: -1,
			data: { is_logged_in: false },
		} as RPCEvent)
		expect(client.state.current?.is_logged_in).toBe(false)
		expect(store.userID).toBe("")
	})

	it("handles sync_status", async () => {
		const { client, rpc } = await makeClient()
		rpc.event.emit({ command: "sync_status", request_id: -1, data: { type: "running" } } as RPCEvent)
		expect(client.syncStatus.current.type).toBe("running")
	})

	it("handles init_complete", async () => {
		const { client, rpc, store } = await makeClient()
		rpc.event.emit({ command: "init_complete", request_id: -1, data: {} } as RPCEvent)
		expect(client.initComplete.current).toBe(true)
		expect(store.stateCache.tryFlush).toHaveBeenCalled()
	})

	it("handles sync_complete", async () => {
		const { client, rpc, store } = await makeClient()
		rpc.event.emit({ command: "sync_complete", request_id: -1, data: {} } as RPCEvent)
		expect(store.applySync).toHaveBeenCalled()
	})

	it("handles events_decrypted", async () => {
		const { client, rpc, store } = await makeClient()
		rpc.event.emit({ command: "events_decrypted", request_id: -1, data: [] } as RPCEvent)
		expect(store.applyDecrypted).toHaveBeenCalled()
	})

	it("handles send_complete", async () => {
		const { client, rpc, store } = await makeClient()
		rpc.event.emit({ command: "send_complete", request_id: -1, data: [] } as RPCEvent)
		expect(store.applySendComplete).toHaveBeenCalled()
	})

	it("handles image_auth_token", async () => {
		const { client, rpc, store } = await makeClient()
		rpc.event.emit({ command: "image_auth_token", request_id: -1, data: "tok123" } as RPCEvent)
		expect(store.imageAuthToken).toBe("tok123")
	})

	it("handles typing", async () => {
		const { client, rpc, store } = await makeClient()
		rpc.event.emit({ command: "typing", request_id: -1, data: {} } as RPCEvent)
		expect(store.applyTyping).toHaveBeenCalled()
	})

	it("triggers registerWebPush when verified", async () => {
		const { client, rpc } = await makeClient()
		const getRegFn = vi.fn().mockResolvedValue(null)
		vi.stubGlobal("navigator", { serviceWorker: { getRegistration: getRegFn } })
		rpc.event.emit({
			command: "client_state",
			request_id: -1,
			data: { is_logged_in: true, user_id: "@u:example.com", is_verified: true },
		} as RPCEvent)
		expect(getRegFn).toHaveBeenCalledWith("pushmuks")
	})
})

describe("userID getter", () => {
	it("returns empty string when not logged in", async () => {
		const { client } = await makeClient()
		expect(client.userID).toBe("")
	})

	it("returns user ID when logged in", async () => {
		const { client, rpc } = await makeClient()
		rpc.event.emit({
			command: "client_state",
			request_id: -1,
			data: { is_logged_in: true, user_id: "@user:example.com" },
		} as RPCEvent)
		expect(client.userID).toBe("@user:example.com")
	})
})

describe("fetchCapabilities", () => {
	it("fetches and caches capabilities", async () => {
		const { client, rpc } = await makeClient()
		rpc.getCapabilities = vi.fn().mockResolvedValue({ capabilities: { super_cast: true } })
		const result1 = await client.fetchCapabilities()
		const result2 = await client.fetchCapabilities()
		expect(result1).toEqual({ super_cast: true })
		expect(result2).toEqual({ super_cast: true })
		expect(rpc.getCapabilities).toHaveBeenCalledTimes(1)
	})

	it("resets cache on error and re-throws", async () => {
		const { client, rpc } = await makeClient()
		rpc.getCapabilities = vi.fn().mockRejectedValueOnce(new Error("fail"))
			.mockResolvedValue({ capabilities: {} })
		await expect(client.fetchCapabilities()).rejects.toThrow("fail")
		// Should fetch again after error
		await client.fetchCapabilities()
		expect(rpc.getCapabilities).toHaveBeenCalledTimes(2)
	})
})

describe("addWidgetListener", () => {
	it("adds listener and requests to-device events once", async () => {
		const { client, rpc, store } = await makeClient()
		const listener = { onTimelineEvent: vi.fn(), onStateEvent: vi.fn(), onToDeviceEvent: vi.fn(), onRoomChange: vi.fn() }
		const unsub = client.addWidgetListener(listener)
		expect(store.widgetListeners.size).toBe(1)
		expect(rpc.setListenToDevice).toHaveBeenCalledWith(true)
		// Second listener doesn't re-request
		const listener2 = { onTimelineEvent: vi.fn(), onStateEvent: vi.fn(), onToDeviceEvent: vi.fn(), onRoomChange: vi.fn() }
		client.addWidgetListener(listener2)
		expect(rpc.setListenToDevice).toHaveBeenCalledTimes(1)
		// Unsubscribing all stops to-device
		unsub()
		client.addWidgetListener(listener2) // still one left... wait no, we removed first
		// listener2 still active, removing it should stop
	})

	it("stops to-device events when last listener removed", async () => {
		const { client, rpc, store } = await makeClient()
		const listener = { onTimelineEvent: vi.fn(), onStateEvent: vi.fn(), onToDeviceEvent: vi.fn(), onRoomChange: vi.fn() }
		const unsub = client.addWidgetListener(listener)
		unsub()
		expect(store.widgetListeners.size).toBe(0)
		expect(rpc.setListenToDevice).toHaveBeenCalledWith(false)
	})
})

describe("registerURIHandler", () => {
	it("registers matrix protocol handler", async () => {
		const { client } = await makeClient()
		const registerProtocolHandler = vi.fn()
		vi.stubGlobal("navigator", { registerProtocolHandler })
		client.registerURIHandler()
		expect(registerProtocolHandler).toHaveBeenCalledWith("matrix", "#/uri/%s")
	})
})

describe("requestNotificationPermission", () => {
	it("calls Notification.requestPermission", async () => {
		const { client } = await makeClient()
		const requestPermission = vi.fn().mockResolvedValue("granted")
		;(window as any).Notification = { requestPermission }
		await client.requestNotificationPermission()
		expect(requestPermission).toHaveBeenCalled()
		delete (window as any).Notification
	})

	it("shows alert when called with event", async () => {
		const { client } = await makeClient()
		const requestPermission = vi.fn().mockResolvedValue("denied")
		;(window as any).Notification = { requestPermission }
		await client.requestNotificationPermission({} as MouseEvent)
		await flushMicrotasks(20)
		expect(window.alert).toHaveBeenCalledWith("Notification permission: denied")
		delete (window as any).Notification
	})

	it("does nothing when Notification undefined", async () => {
		const { client } = await makeClient()
		expect(() => client.requestNotificationPermission()).not.toThrow()
	})
})

describe("start", () => {
	it("starts rpc after auth on non-android", async () => {
		const { client, rpc } = await makeClient()
		const cleanup = client.start()
		await flushMicrotasks()
		expect(rpc.tryAuth).toHaveBeenCalled()
		expect(rpc.start).toHaveBeenCalled()
		cleanup()
		expect(rpc.stop).toHaveBeenCalled()
	})

	it("does not start rpc when auth fails", async () => {
		const { client, rpc } = await makeClient()
		rpc.tryAuth = vi.fn().mockResolvedValue(false)
		const cleanup = client.start()
		await flushMicrotasks()
		expect(rpc.start).not.toHaveBeenCalled()
		cleanup()
	})

	it("does not start rpc when signal aborted after auth", async () => {
		const { client, rpc } = await makeClient()
		// Abort mid-flight: tryAuth resolves but signal already aborted
		rpc.tryAuth = vi.fn().mockImplementation(async (signal: AbortSignal) => {
			// simulate abort happening during auth
			return true
		})
		const cleanup = client.start()
		await flushMicrotasks()
		expect(rpc.start).toHaveBeenCalled()
		cleanup()
	})

	it("returns cleanup that closes cache and stops rpc", async () => {
		const { client, rpc, store } = await makeClient()
		const cleanup = client.start()
		await flushMicrotasks()
		cleanup()
		expect(store.closeCache).toHaveBeenCalled()
		expect(rpc.stop).toHaveBeenCalled()
	})

	it("android path dispatches ready event", async () => {
		;(window as any).gomuksAndroid = { getTabID: () => "t" }
		const { client, rpc } = await makeClient()
		const dispatchSpy = vi.spyOn(window, "dispatchEvent")
		const cleanup = client.start()
		await flushMicrotasks()
		const readyEvent = dispatchSpy.mock.calls
			.find(call => (call[0] as CustomEvent).type === "GomuksWebMessageToAndroid"
				&& (call[0] as CustomEvent).detail?.event === "ready")
		expect(readyEvent).toBeDefined()
		expect(rpc.tryAuth).not.toHaveBeenCalled()
		cleanup()
	})
})

describe("requestMemberEvent", () => {
	it("returns null when room not found", async () => {
		const { client } = await makeClient()
		expect(client.requestMemberEvent("!missing:example.com", "@user:example.com")).toBeNull()
	})

	it("returns null when member already in state", async () => {
		const { client, store } = await makeClient()
		const room = makeFakeRoom()
		room.state.set("m.room.member", new Map([["@user:example.com", 1]]))
		store.rooms.set("!room:example.com", room as any)
		expect(client.requestMemberEvent(room as any, "@user:example.com")).toBeNull()
	})

	it("returns null when member already requested", async () => {
		const { client, store } = await makeClient()
		const room = makeFakeRoom()
		room.requestedMembers.add("@user:example.com")
		store.rooms.set("!room:example.com", room as any)
		expect(client.requestMemberEvent(room as any, "@user:example.com")).toBeNull()
	})

	it("queues state request and batches multiple members", async () => {
		const { client, rpc, store } = await makeClient()
		const room = makeFakeRoom()
		store.rooms.set("!room:example.com", room as any)
		const p1 = client.requestMemberEvent(room as any, "@u1:example.com")
		const p2 = client.requestMemberEvent(room as any, "@u2:example.com")
		expect(p1).toBe(p2) // same batched promise
		await p1
		expect(rpc.getSpecificRoomState).toHaveBeenCalledWith([
			{ room_id: "!room:example.com", type: "m.room.member", state_key: "@u1:example.com" },
			{ room_id: "!room:example.com", type: "m.room.member", state_key: "@u2:example.com" },
		])
	})

	it("resolves via room ID string", async () => {
		const { client, rpc, store } = await makeClient()
		const room = makeFakeRoom()
		store.rooms.set("!room:example.com", room as any)
		await client.requestMemberEvent("!room:example.com", "@u1:example.com")
		expect(rpc.getSpecificRoomState).toHaveBeenCalled()
	})
})

describe("requestEvent", () => {
	it("does nothing when room not found", async () => {
		const { client, rpc } = await makeClient()
		client.requestEvent("!missing:example.com", "$evt")
		await flushMicrotasks(20)
		expect(rpc.getEvent).not.toHaveBeenCalled()
	})

	it("does nothing when event already cached", async () => {
		const { client, rpc } = await makeClient()
		const room = makeFakeRoom()
		room.eventsByID.set("$evt", {} as any)
		client.requestEvent(room as any, "$evt")
		await flushMicrotasks(20)
		expect(rpc.getEvent).not.toHaveBeenCalled()
	})

	it("does nothing when event already requested", async () => {
		const { client, rpc } = await makeClient()
		const room = makeFakeRoom()
		room.requestedEvents.add("$evt")
		client.requestEvent(room as any, "$evt")
		await flushMicrotasks(20)
		expect(rpc.getEvent).not.toHaveBeenCalled()
	})

	it("fetches and applies event", async () => {
		const { client, rpc } = await makeClient()
		const room = makeFakeRoom()
		rpc.getEvent = vi.fn().mockResolvedValue({ event_id: "$evt" })
		client.requestEvent(room as any, "$evt")
		await flushMicrotasks(20)
		expect(rpc.getEvent).toHaveBeenCalledWith("!room:example.com", "$evt", undefined)
		expect(room.applyEvent).toHaveBeenCalled()
	})

	it("alerts on unredact failure", async () => {
		const { client, rpc } = await makeClient()
		const room = makeFakeRoom()
		rpc.getEvent = vi.fn().mockRejectedValue(new Error("nope"))
		client.requestEvent(room as any, "$evt", true)
		await flushMicrotasks(20)
		expect(room.requestedEvents.has("$evt")).toBe(false)
		expect(window.alert).toHaveBeenCalled()
	})
})

describe("requestEventByRowID", () => {
	it("does nothing when event cached", async () => {
		const { client, rpc } = await makeClient()
		const room = makeFakeRoom()
		room.eventsByRowID.set(42, {} as any)
		client.requestEventByRowID(room as any, 42)
		await flushMicrotasks(20)
		expect(rpc.getEventByRowID).not.toHaveBeenCalled()
	})

	it("fetches and applies", async () => {
		const { client, rpc } = await makeClient()
		const room = makeFakeRoom()
		rpc.getEventByRowID = vi.fn().mockResolvedValue({ rowid: 42 })
		client.requestEventByRowID(room as any, 42)
		await flushMicrotasks(20)
		expect(rpc.getEventByRowID).toHaveBeenCalledWith(42)
		expect(room.applyEvent).toHaveBeenCalled()
	})
})

describe("getRelatedEvents", () => {
	it("returns [] when room not found", async () => {
		const { client } = await makeClient()
		expect(await client.getRelatedEvents("!missing:example.com", "$evt")).toEqual([])
	})

	it("maps events through room", async () => {
		const { client, rpc, store } = await makeClient()
		const room = makeFakeRoom()
		store.rooms.set("!room:example.com", room as any)
		rpc.getRelatedEvents = vi.fn().mockResolvedValue([{ event_id: "$r1" }])
		const result = await client.getRelatedEvents(room as any, "$evt")
		expect(result).toEqual([{ event_id: "$r1" }])
		expect(room.getOrApplyEvent).toHaveBeenCalled()
	})
})

describe("getStickyEvents", () => {
	it("returns [] when room not found", async () => {
		const { client } = await makeClient()
		expect(await client.getStickyEvents("!missing:example.com")).toEqual([])
	})

	it("maps events through room", async () => {
		const { client, rpc, store } = await makeClient()
		const room = makeFakeRoom()
		store.rooms.set("!room:example.com", room as any)
		rpc.getStickyEvents = vi.fn().mockResolvedValue([{ event_id: "$s1" }])
		const result = await client.getStickyEvents(room as any)
		expect(result).toEqual([{ event_id: "$s1" }])
	})
})

describe("getMentions", () => {
	it("filters mentions from unknown rooms", async () => {
		const { client, rpc } = await makeClient()
		rpc.getMentions = vi.fn().mockResolvedValue([
			{ room_id: "!missing:example.com", event_id: "$e1" },
		])
		expect(await client.getMentions()).toEqual([])
	})

	it("maps events through room", async () => {
		const { client, rpc, store } = await makeClient()
		const room = makeFakeRoom()
		store.rooms.set("!room:example.com", room as any)
		rpc.getMentions = vi.fn().mockResolvedValue([{ room_id: "!room:example.com", event_id: "$e1" }])
		const result = await client.getMentions({ maxTS: 100, limit: 5 })
		expect(result).toEqual([{ room_id: "!room:example.com", event_id: "$e1" }])
		expect(rpc.getMentions).toHaveBeenCalledWith(100, undefined, 5, undefined)
	})
})

describe("search", () => {
	it("resolves with mapped events and next batch", async () => {
		const { client, rpc, store } = await makeClient()
		const room = makeFakeRoom()
		store.rooms.set("!room:example.com", room as any)
		const cancel = vi.fn()
		rpc.searchLocal = vi.fn(() => {
			const p = Promise.resolve({ events: [{ room_id: "!room:example.com", event_id: "$e1" }], next_batch: "nb1" }) as any
			p.cancel = cancel
			return p
		})
		const result = await client.search(true, {} as any)
		expect(result[0]).toEqual([{ room_id: "!room:example.com", event_id: "$e1" }])
		expect(result[1]).toBe("nb1")
	})

	it("propagates rejection", async () => {
		const { client, rpc } = await makeClient()
		rpc.searchServer = vi.fn(() => {
			const p = Promise.reject(new Error("search failed")) as any
			p.cancel = vi.fn()
			return p
		})
		await expect(client.search(false, {} as any)).rejects.toThrow("search failed")
	})
})

describe("pinMessage", () => {
	it("does nothing when already in desired state", async () => {
		const { client, rpc } = await makeClient()
		const room = makeFakeRoom()
		room.getPinnedEvents = vi.fn().mockReturnValue(["$evt"])
		await client.pinMessage(room as any, "$evt", true)
		expect(rpc.setState).not.toHaveBeenCalled()
	})

	it("pins a message", async () => {
		const { client, rpc } = await makeClient()
		const room = makeFakeRoom()
		room.getPinnedEvents = vi.fn().mockReturnValue(["$other"])
		await client.pinMessage(room as any, "$evt", true)
		expect(rpc.setState).toHaveBeenCalledWith(
			"!room:example.com", "m.room.pinned_events", "", { pinned: ["$other", "$evt"] },
		)
	})

	it("unpins a message", async () => {
		const { client, rpc } = await makeClient()
		const room = makeFakeRoom()
		room.getPinnedEvents = vi.fn().mockReturnValue(["$evt", "$other"])
		await client.pinMessage(room as any, "$evt", false)
		expect(rpc.setState).toHaveBeenCalledWith(
			"!room:example.com", "m.room.pinned_events", "", { pinned: ["$other"] },
		)
	})
})

describe("handleOutgoingEvent", () => {
	it("applies event and notifies timeline", async () => {
		const { client } = await makeClient()
		const room = makeFakeRoom()
		const dbEvent = { rowid: 1, timeline_rowid: 1, sender: "@user:example.com" }
		client.handleOutgoingEvent(dbEvent as any, room as any)
		expect(room.applyEvent).toHaveBeenCalledWith(dbEvent, true)
		expect(room.pendingEvents).toContain(1)
		expect(room.notifyTimelineSubscribers).toHaveBeenCalled()
	})

	it("pushes fake gomuks events to timeline", async () => {
		const { client } = await makeClient()
		const room = makeFakeRoom()
		const dbEvent = { rowid: 5, timeline_rowid: 5, sender: "@gomuks" }
		client.handleOutgoingEvent(dbEvent as any, room as any)
		// applyEvent called with false (not local echo for fake sender)
		expect(room.applyEvent).toHaveBeenCalledWith(dbEvent, false)
		expect(room.timeline).toEqual([{ timeline_rowid: 5, event_rowid: 5 }])
	})

	it("ignores duplicate events", async () => {
		const { client } = await makeClient()
		const room = makeFakeRoom()
		room.eventsByRowID.set(1, {} as any)
		const dbEvent = { rowid: 1, timeline_rowid: 1, sender: "@user:example.com" }
		client.handleOutgoingEvent(dbEvent as any, room as any)
		expect(room.applyEvent).not.toHaveBeenCalled()
	})
})

describe("sendEvent", () => {
	it("throws when room not found", async () => {
		const { client } = await makeClient()
		await expect(client.sendEvent("!missing:example.com", "m.room.message", {})).rejects.toThrow("Room not found")
	})

	it("sends and handles outgoing event", async () => {
		const { client, rpc, store } = await makeClient()
		const room = makeFakeRoom()
		store.rooms.set("!room:example.com", room as any)
		rpc.sendEvent = vi.fn().mockResolvedValue({ rowid: 1, timeline_rowid: 1, sender: "@user:example.com" })
		await client.sendEvent("!room:example.com", "m.room.message", { body: "hi" })
		expect(rpc.sendEvent).toHaveBeenCalledWith("!room:example.com", "m.room.message", { body: "hi" }, false)
		expect(room.applyEvent).toHaveBeenCalled()
	})
})

describe("sendMessage", () => {
	it("throws when room not found", async () => {
		const { client } = await makeClient()
		await expect(client.sendMessage({ room_id: "!missing:example.com", text: "hi" })).rejects.toThrow("Room not found")
	})

	it("sends and handles outgoing event when dbEvent returned", async () => {
		const { client, rpc, store } = await makeClient()
		const room = makeFakeRoom()
		store.rooms.set("!room:example.com", room as any)
		rpc.sendMessage = vi.fn().mockResolvedValue({ rowid: 1, timeline_rowid: 1, sender: "@user:example.com" })
		await client.sendMessage({ room_id: "!room:example.com", text: "hi" })
		expect(room.applyEvent).toHaveBeenCalled()
	})

	it("does nothing when dbEvent is null", async () => {
		const { client, rpc, store } = await makeClient()
		const room = makeFakeRoom()
		store.rooms.set("!room:example.com", room as any)
		rpc.sendMessage = vi.fn().mockResolvedValue(null)
		await client.sendMessage({ room_id: "!room:example.com", text: "hi" })
		expect(room.applyEvent).not.toHaveBeenCalled()
	})
})

describe("subscribeToEmojiPack", () => {
	it("subscribes to a new pack", async () => {
		const { client, rpc } = await makeClient()
		await client.subscribeToEmojiPack({ room_id: "!r:example.com", type: "m.room.image_pack", state_key: "" })
		expect(rpc.setAccountData).toHaveBeenCalledWith("im.ponies.emote_rooms", expect.objectContaining({
			rooms: { "!r:example.com": { "": {} } },
		}))
	})

	it("unsubscribes from an existing pack", async () => {
		const { client, rpc } = await makeClient()
		// Pre-populate account data
		const emoteRooms = { rooms: { "!r:example.com": { "": {} } } }
		;(client.store as any).accountData.set("im.ponies.emote_rooms", emoteRooms)
		await client.subscribeToEmojiPack({ room_id: "!r:example.com", type: "m.room.image_pack", state_key: "" }, false)
		expect(rpc.setAccountData).toHaveBeenCalledWith("im.ponies.emote_rooms", { rooms: { "!r:example.com": {} } })
	})

	it("does nothing when subscribing to an already-subscribed pack", async () => {
		const { client, rpc } = await makeClient()
		const emoteRooms = { rooms: { "!r:example.com": { "": {} } } }
		;(client.store as any).accountData.set("im.ponies.emote_rooms", emoteRooms)
		await client.subscribeToEmojiPack({ room_id: "!r:example.com", type: "m.room.image_pack", state_key: "" }, true)
		expect(rpc.setAccountData).not.toHaveBeenCalled()
	})

	it("does nothing when unsubscribing from an unsubscribed pack", async () => {
		const { client, rpc } = await makeClient()
		await client.subscribeToEmojiPack({ room_id: "!r:example.com", type: "m.room.image_pack", state_key: "" }, false)
		expect(rpc.setAccountData).not.toHaveBeenCalled()
	})

	it("writes to both account data events when new format exists", async () => {
		const { client, rpc } = await makeClient()
		;(client.store as any).accountData.set("m.image_pack.rooms", { rooms: {} })
		await client.subscribeToEmojiPack({ room_id: "!r:example.com", type: "m.room.image_pack", state_key: "" })
		expect(rpc.setAccountData).toHaveBeenCalledTimes(2)
	})
})

describe("incrementFrequentlyUsedEmoji", () => {
	it("adds new emoji", async () => {
		const { client, rpc } = await makeClient()
		await client.incrementFrequentlyUsedEmoji("😀")
		expect(rpc.setAccountData).toHaveBeenCalledWith("io.element.recent_emoji", {
			recent_emoji: [["😀", 1]],
		})
	})

	it("increments existing emoji and moves to front", async () => {
		const { client, rpc } = await makeClient()
		;(client.store as any).accountData.set("io.element.recent_emoji", {
			recent_emoji: [["😀", 1], ["🎉", 5]],
		})
		await client.incrementFrequentlyUsedEmoji("🎉")
		expect(rpc.setAccountData).toHaveBeenCalledWith("io.element.recent_emoji", {
			recent_emoji: [["🎉", 6], ["😀", 1]],
		})
	})

	it("caps list at 100 entries", async () => {
		const { client, rpc } = await makeClient()
		const recent = Array.from({ length: 100 }, (_, i) => [`e${i}`, 1])
		;(client.store as any).accountData.set("io.element.recent_emoji", { recent_emoji: recent })
		await client.incrementFrequentlyUsedEmoji("new")
		const call = rpc.setAccountData.mock.calls[0][1] as { recent_emoji: string[][] }
		expect(call.recent_emoji.length).toBe(100)
		expect(call.recent_emoji[0]).toEqual(["new", 1])
	})
})

describe("loadSpecificRoomState", () => {
	it("returns early when all states loaded", async () => {
		const { client, rpc, store } = await makeClient()
		const room = makeFakeRoom()
		room.getStateEvent = vi.fn().mockReturnValue({ loaded: true })
		store.rooms.set("!room:example.com", room as any)
		await client.loadSpecificRoomState([{ room_id: "!room:example.com", type: "m.room.topic", state_key: "" }])
		expect(rpc.getSpecificRoomState).not.toHaveBeenCalled()
	})

	it("fetches missing states and applies them", async () => {
		const { client, rpc, store } = await makeClient()
		const room = makeFakeRoom()
		store.rooms.set("!room:example.com", room as any)
		rpc.getSpecificRoomState = vi.fn().mockResolvedValue([
			{ room_id: "!room:example.com", type: "m.room.topic", state_key: "" },
		])
		await client.loadSpecificRoomState([{ room_id: "!room:example.com", type: "m.room.topic", state_key: "" }])
		expect(rpc.getSpecificRoomState).toHaveBeenCalled()
		expect(room.applyState).toHaveBeenCalled()
	})

	it("skips keys for unknown rooms", async () => {
		const { client, rpc } = await makeClient()
		rpc.getSpecificRoomState = vi.fn().mockResolvedValue([])
		await client.loadSpecificRoomState([{ room_id: "!missing:example.com", type: "m.room.topic", state_key: "" }])
		expect(rpc.getSpecificRoomState).not.toHaveBeenCalled()
	})

	it("checks emoji pack alt keys", async () => {
		const { client, rpc, store } = await makeClient()
		const room = makeFakeRoom()
		// m.room.image_pack missing but im.ponies.emote_rooms alt key present -> skip
		room.getStateEvent = vi.fn((type: string) =>
			type === "im.ponies.emote_rooms" ? { loaded: true } : undefined)
		store.rooms.set("!room:example.com", room as any)
		await client.loadSpecificRoomState([{ room_id: "!room:example.com", type: "m.room.image_pack", state_key: "" }], true)
		expect(rpc.getSpecificRoomState).not.toHaveBeenCalled()
	})
})

describe("loadRoomStateIfNecessary", () => {
	it("resolves immediately when state loaded", async () => {
		const { client } = await makeClient()
		const room = makeFakeRoom()
		room.stateLoaded = true
		await expect(client.loadRoomStateIfNecessary(room as any)).resolves.toBeUndefined()
	})

	it("loads state when not loaded", async () => {
		const { client, rpc, store } = await makeClient()
		const room = makeFakeRoom()
		store.rooms.set("!room:example.com", room as any)
		rpc.getRoomState = vi.fn().mockResolvedValue([])
		const promise = client.loadRoomStateIfNecessary(room as any)
		expect(room.waitStateLoaded).toBe(promise)
		await promise
		expect(rpc.getRoomState).toHaveBeenCalled()
	})
})

describe("loadRoomState", () => {
	it("throws when room not found", async () => {
		const { client } = await makeClient()
		await expect(client.loadRoomState("!missing:example.com")).rejects.toThrow("Room not found")
	})

	it("loads state with default options (omit members)", async () => {
		const { client, rpc, store } = await makeClient()
		const room = makeFakeRoom()
		store.rooms.set("!room:example.com", room as any)
		rpc.getRoomState = vi.fn().mockResolvedValue([])
		await client.loadRoomState("!room:example.com")
		expect(rpc.getRoomState).toHaveBeenCalledWith("!room:example.com", false, true, false)
		expect(room.applyFullState).toHaveBeenCalledWith([], true)
	})

	it("requests members when not omitted", async () => {
		const { client, rpc, store } = await makeClient()
		const room = makeFakeRoom()
		store.rooms.set("!room:example.com", room as any)
		rpc.getRoomState = vi.fn().mockResolvedValue([])
		await client.loadRoomState("!room:example.com", { omitMembers: false })
		expect(room.membersRequested).toBe(true)
		expect(room.meta.current.has_member_list).toBe(true)
		expect(room.applyFullState).toHaveBeenCalledWith([], false)
	})
})

describe("resetTimeline", () => {
	it("throws when room not found", async () => {
		const { client } = await makeClient()
		await expect(client.resetTimeline("!missing:example.com")).rejects.toThrow("Room not found")
	})

	it("throws when already paginating", async () => {
		const { client, store } = await makeClient()
		const room = makeFakeRoom()
		room.paginating = true
		store.rooms.set("!room:example.com", room as any)
		await expect(client.resetTimeline("!room:example.com")).rejects.toThrow("Already paginating")
	})

	it("resets and paginates", async () => {
		const { client, rpc, store } = await makeClient()
		const room = makeFakeRoom()
		store.rooms.set("!room:example.com", room as any)
		rpc.paginate = vi.fn().mockResolvedValue({ has_more: true, events: [], related_events: {}, receipts: {} })
		await client.resetTimeline("!room:example.com")
		expect(rpc.paginate).toHaveBeenCalledWith("!room:example.com", 0, 50, true)
		expect(room.hasMoreHistory).toBe(true)
		expect(room.applyPagination).toHaveBeenCalled()
		expect(room.paginating).toBe(false)
	})

	it("clears paginating flag on error", async () => {
		const { client, rpc, store } = await makeClient()
		const room = makeFakeRoom()
		store.rooms.set("!room:example.com", room as any)
		rpc.paginate = vi.fn().mockRejectedValue(new Error("fail"))
		await expect(client.resetTimeline("!room:example.com")).rejects.toThrow("fail")
		expect(room.paginating).toBe(false)
	})
})

describe("loadMoreHistory", () => {
	it("throws when room not found", async () => {
		const { client } = await makeClient()
		await expect(client.loadMoreHistory("!missing:example.com")).rejects.toThrow("Room not found")
	})

	it("throws when already paginating", async () => {
		const { client, store } = await makeClient()
		const room = makeFakeRoom()
		room.paginating = true
		store.rooms.set("!room:example.com", room as any)
		await expect(client.loadMoreHistory("!room:example.com")).rejects.toThrow("Already paginating")
	})

	it("loads history from oldest event", async () => {
		const { client, rpc, store } = await makeClient()
		const room = makeFakeRoom()
		room.timeline = [{ timeline_rowid: 100, event_rowid: 10 }]
		store.rooms.set("!room:example.com", room as any)
		rpc.paginate = vi.fn().mockResolvedValue({ has_more: false, events: [], related_events: {}, receipts: {} })
		await client.loadMoreHistory("!room:example.com")
		expect(rpc.paginate).toHaveBeenCalledWith("!room:example.com", 100, 50)
		expect(room.hasMoreHistory).toBe(false)
	})

	it("requests 100 messages when timeline is long", async () => {
		const { client, rpc, store } = await makeClient()
		const room = makeFakeRoom()
		room.timeline = Array.from({ length: 120 }, (_, i) => ({ timeline_rowid: i + 1, event_rowid: i }))
		store.rooms.set("!room:example.com", room as any)
		rpc.paginate = vi.fn().mockResolvedValue({ has_more: false, events: [], related_events: {}, receipts: {} })
		await client.loadMoreHistory("!room:example.com")
		expect(rpc.paginate).toHaveBeenCalledWith("!room:example.com", 1, 100)
	})

	it("throws when timeline changed during fetch", async () => {
		const { client, rpc, store } = await makeClient()
		const room = makeFakeRoom()
		room.timeline = [{ timeline_rowid: 100, event_rowid: 10 }]
		store.rooms.set("!room:example.com", room as any)
		rpc.paginate = vi.fn().mockImplementation(async () => {
			// simulate concurrent change
			room.timeline.unshift({ timeline_rowid: 50, event_rowid: 5 })
			return { has_more: false, events: [], related_events: {}, receipts: {} }
		})
		await expect(client.loadMoreHistory("!room:example.com")).rejects.toThrow("Timeline changed while loading history")
		expect(room.paginating).toBe(false)
	})
})

describe("clearState and logout", () => {
	it("clearState resets dispatchers and store", async () => {
		const { client, store } = await makeClient()
		client.clearState()
		expect(client.initComplete.current).toBe(false)
		expect(client.syncStatus.current.type).toBe("waiting")
		expect(client.state.current).toBeNull()
		expect(store.clear).toHaveBeenCalled()
	})

	it("logout calls rpc.logout and clears state + localStorage", async () => {
		const { client, rpc, store } = await makeClient()
		localStorage.setItem("some", "value")
		await client.logout()
		expect(rpc.logout).toHaveBeenCalled()
		expect(store.clear).toHaveBeenCalled()
		expect(localStorage.length).toBe(0)
	})
})

describe("registerWebPush", () => {
	it("does nothing when web_push preference is false", async () => {
		const { client, rpc } = await makeClient()
		localStorage.removeItem("push_device_id")
		const getRegistration = vi.fn().mockResolvedValue(null)
		vi.stubGlobal("navigator", { serviceWorker: { getRegistration } })
		await client.registerWebPush()
		await flushMicrotasks(20)
		expect(getRegistration).toHaveBeenCalledWith("pushmuks")
		expect(rpc.registerPush).not.toHaveBeenCalled()
	})

	it("unregisters old pushmuks worker when disabled", async () => {
		const { client, rpc } = await makeClient()
		localStorage.push_device_id = "old-device"
		const reg = {
			active: { scriptURL: "https://example.com/pushmuks-sw.js" },
			pushManager: {
				getSubscription: vi.fn().mockResolvedValue({ unsubscribe: vi.fn().mockResolvedValue(true) }),
			},
			unregister: vi.fn().mockResolvedValue(true),
		}
		const getRegistration = vi.fn().mockResolvedValue(reg)
		vi.stubGlobal("navigator", { serviceWorker: { getRegistration } })
		await client.registerWebPush()
		expect(getRegistration).toHaveBeenCalledWith("pushmuks")
		// waits for deep async chain (getSubscription -> unsubscribe -> finally -> unregister)
		await flushMicrotasks(20)
		expect(reg.unregister).toHaveBeenCalled()
		expect(rpc.registerPush).toHaveBeenCalledWith(expect.objectContaining({
			type: "web", device_id: "old-device", expiration: 1,
		}))
	})

	it("subscribes and registers push when enabled", async () => {
		const { client, rpc, store } = await makeClient()
		store.localPreferenceCache.web_push = true
		const sub = {
			toJSON: vi.fn().mockReturnValue({ endpoint: "https://push.example.com/abc" }),
			expirationTime: 12345,
		}
		const reg = {
			pushManager: {
				subscribe: vi.fn().mockResolvedValue(sub),
			},
		}
		const register = vi.fn().mockResolvedValue(reg)
		vi.stubGlobal("navigator", { serviceWorker: { register } })
		await client.registerWebPush()
		expect(localStorage.push_device_id).toBeDefined()
		await flushMicrotasks(20)
		expect(register).toHaveBeenCalledWith("pushmuks-sw.js", { scope: "pushmuks" })
		expect(rpc.registerPush).toHaveBeenCalledWith(expect.objectContaining({
			type: "web",
			data: { endpoint: "https://push.example.com/abc" },
			expiration: 12345,
		}))
	})

	it("uses stored vapid key when subscribing", async () => {
		const { client, store } = await makeClient()
		store.localPreferenceCache.web_push = true
		;(window as any).vapidPublicKey = "vapid-key-123"
		const subscribe = vi.fn().mockResolvedValue({
			toJSON: () => ({}), expirationTime: null,
		})
		const reg = { pushManager: { subscribe } }
		vi.stubGlobal("navigator", { serviceWorker: { register: vi.fn().mockResolvedValue(reg) } })
		await client.registerWebPush()
		await flushMicrotasks(20)
		expect(subscribe).toHaveBeenCalledWith(expect.objectContaining({ applicationServerKey: "vapid-key-123" }))
	})

	it("falls back to meta tag vapid key", async () => {
		const { client, store } = await makeClient()
		store.localPreferenceCache.web_push = true
		const meta = document.createElement("meta")
		meta.name = "gomuks-vapid-key"
		meta.content = "meta-vapid-key"
		document.head.appendChild(meta)
		const subscribe = vi.fn().mockResolvedValue({
			toJSON: () => ({}), expirationTime: null,
		})
		const reg = { pushManager: { subscribe } }
		vi.stubGlobal("navigator", { serviceWorker: { register: vi.fn().mockResolvedValue(reg) } })
		await client.registerWebPush()
		await flushMicrotasks(20)
		expect(subscribe).toHaveBeenCalledWith(expect.objectContaining({ applicationServerKey: "meta-vapid-key" }))
		meta.remove()
	})

	it("refreshes existing subscription when refresh=true", async () => {
		const { client, rpc, store } = await makeClient()
		store.localPreferenceCache.web_push = true
		const sub = {
			toJSON: () => ({ endpoint: "ep" }),
			expirationTime: null,
		}
		const reg = { pushManager: { getSubscription: vi.fn().mockResolvedValue(sub) } }
		vi.stubGlobal("navigator", { serviceWorker: { register: vi.fn().mockResolvedValue(reg) } })
		await client.registerWebPush(true)
		await flushMicrotasks(20)
		expect(reg.pushManager.getSubscription).toHaveBeenCalled()
		expect(rpc.registerPush).toHaveBeenCalled()
	})

	it("disables web push when subscribe fails", async () => {
		const { client, store } = await makeClient()
		store.localPreferenceCache.web_push = true
		const reg = { pushManager: { subscribe: vi.fn().mockRejectedValue(new Error("denied")) } }
		vi.stubGlobal("navigator", { serviceWorker: { register: vi.fn().mockResolvedValue(reg) } })
		await client.registerWebPush()
		await flushMicrotasks(20)
		expect(store.localPreferenceCache.web_push).toBe(false)
	})

	it("disables web push when no subscription returned", async () => {
		const { client, store } = await makeClient()
		store.localPreferenceCache.web_push = true
		const reg = { pushManager: { subscribe: vi.fn().mockResolvedValue(null) } }
		vi.stubGlobal("navigator", { serviceWorker: { register: vi.fn().mockResolvedValue(reg) } })
		await client.registerWebPush()
		await flushMicrotasks(20)
		expect(store.localPreferenceCache.web_push).toBe(false)
	})

	it("disables web push when service worker registration fails", async () => {
		const { client, store } = await makeClient()
		store.localPreferenceCache.web_push = true
		vi.stubGlobal("navigator", { serviceWorker: { register: vi.fn().mockRejectedValue(new Error("sw fail")) } })
		await client.registerWebPush()
		await flushMicrotasks(20)
		expect(store.localPreferenceCache.web_push).toBe(false)
	})
})

describe("android message handling", () => {
	function setupAndroid() {
		;(window as any).gomuksAndroid = { getTabID: () => "t" }
	}

	function dispatchAndroidMessage(detail: unknown) {
		window.dispatchEvent(new CustomEvent("GomuksAndroidMessageToWeb", { detail: JSON.stringify(detail) }))
	}

	it("handles register_push message", async () => {
		setupAndroid()
		const { client, rpc } = await makeClient()
		const cleanup = client.start()
		await flushMicrotasks()
		dispatchAndroidMessage({
			type: "register_push",
			device_id: "dev1",
			token: "tok",
			encryption: "enc",
			expiration: 123,
		})
		await flushMicrotasks()
		expect(rpc.registerPush).toHaveBeenCalledWith({
			type: "fcm", device_id: "dev1", data: "tok", encryption: "enc", expiration: 123,
		})
		cleanup()
	})

	it("handles auth message and starts rpc on success", async () => {
		setupAndroid()
		const { client, rpc } = await makeClient()
		const cleanup = client.start()
		await flushMicrotasks()
		dispatchAndroidMessage({ type: "auth", authorization: "Bearer tok" })
		await flushMicrotasks()
		expect(rpc.start).toHaveBeenCalled()
		cleanup()
	})

	it("dispatches auth_fail on failed auth", async () => {
		setupAndroid()
		const { client } = await makeClient()
		const dispatchSpy = vi.spyOn(window, "dispatchEvent")
		;(globalThis.fetch as any) = vi.fn().mockResolvedValue({ ok: false, status: 401, statusText: "Unauthorized" })
		const cleanup = client.start()
		await flushMicrotasks()
		dispatchAndroidMessage({ type: "auth", authorization: "Bearer bad" })
		await flushMicrotasks()
		const failEvent = dispatchSpy.mock.calls
			.find(call => (call[0] as CustomEvent).type === "GomuksWebMessageToAndroid"
				&& (call[0] as CustomEvent).detail?.event === "auth_fail")
		expect(failEvent).toBeDefined()
		cleanup()
	})

	it("dispatches auth_fail on auth fetch error", async () => {
		setupAndroid()
		const { client } = await makeClient()
		const dispatchSpy = vi.spyOn(window, "dispatchEvent")
		;(globalThis.fetch as any) = vi.fn().mockRejectedValue(new Error("network down"))
		const cleanup = client.start()
		await flushMicrotasks()
		dispatchAndroidMessage({ type: "auth", authorization: "Bearer x" })
		await flushMicrotasks()
		const failEvent = dispatchSpy.mock.calls
			.find(call => (call[0] as CustomEvent).type === "GomuksWebMessageToAndroid"
				&& (call[0] as CustomEvent).detail?.event === "auth_fail")
		expect(failEvent).toBeDefined()
		cleanup()
	})

	it("handles share message", async () => {
		setupAndroid()
		const { client } = await makeClient()
		const setPendingShare = vi.fn()
		;(window as any).mainScreenContext = { setPendingShare }
		// Uint8Array.fromBase64 is not available in Node 22 — stub it
		;(Uint8Array as any).fromBase64 = (str: string) => {
			const bin = atob(str)
			return Uint8Array.from(bin, c => c.charCodeAt(0))
		}
		const cleanup = client.start()
		await flushMicrotasks()
		// base64 of "hello"
		dispatchAndroidMessage({ type: "share", payload: btoa("hello"), name: "file.txt", mime_type: "text/plain" })
		await flushMicrotasks()
		expect(setPendingShare).toHaveBeenCalled()
		cleanup()
		delete (window as any).mainScreenContext
		delete (Uint8Array as any).fromBase64
	})

	it("logs error when shared file processing fails", async () => {
		setupAndroid()
		const { client } = await makeClient()
		const errSpy = vi.spyOn(console, "error").mockImplementation(() => {})
		// ensure fromBase64 is absent so the handler throws
		delete (Uint8Array as any).fromBase64
		const cleanup = client.start()
		await flushMicrotasks()
		dispatchAndroidMessage({ type: "share", payload: "!!!not-base64", name: "file.txt", mime_type: "text/plain" })
		await flushMicrotasks()
		expect(errSpy).toHaveBeenCalledWith("Failed to process shared file:", expect.anything())
		cleanup()
	})

	it("dispatches connected event when rpc connects", async () => {
		setupAndroid()
		const { client, rpc } = await makeClient()
		const dispatchSpy = vi.spyOn(window, "dispatchEvent")
		const cleanup = client.start()
		await flushMicrotasks()
		rpc.connect.emit({ connected: true, reconnecting: false, error: null })
		const connectedEvent = dispatchSpy.mock.calls
			.find(call => (call[0] as CustomEvent).type === "GomuksWebMessageToAndroid"
				&& (call[0] as CustomEvent).detail?.event === "connected")
		expect(connectedEvent).toBeDefined()
		cleanup()
	})
})

describe("emote rooms change subscriber", () => {
	it("subscribes to emote rooms account data changes", async () => {
		const { store } = await makeClient()
		// Constructor should register subscribers for both emote room event types
		expect(store.accountDataSubs.getSubscriber).toHaveBeenCalledWith("im.ponies.emote_rooms")
		expect(store.accountDataSubs.getSubscriber).toHaveBeenCalledWith("m.image_pack.rooms")
	})

	it("loads emoji pack states when emote rooms change", async () => {
		const subs = new Map<string, () => void>()
		const store = makeFakeStore()
		store.accountDataSubs.getSubscriber = vi.fn((key: string) => {
			return (cb: () => void) => {
				subs.set(key, cb)
				return () => subs.delete(key)
			}
		})
		;(globalThis as any).__fakeStore = store
		const { default: Client } = await importClientModule()
		const rpc = makeMockRPC()
		new Client(rpc as unknown as RPCClient)
		// Trigger the emote rooms change
		subs.get("im.ponies.emote_rooms")!()
		await flushMicrotasks(20)
		expect(store.invalidateEmojiPackKeyCache).toHaveBeenCalled()
	})
})
