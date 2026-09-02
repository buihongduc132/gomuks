import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import zlib from "node:zlib"
import WSClient, { checkUpdate } from "./wsclient"
import type { RPCCommand } from "./types"

// Mock WebSocket
class FakeWebSocket {
	static instances: FakeWebSocket[] = []
	static CONNECTING = 0
	static OPEN = 1
	static CLOSING = 2
	static CLOSED = 3

	readyState = FakeWebSocket.CONNECTING
	binaryType = ""
	url: string
	onmessage: ((ev: MessageEvent) => void) | null = null
	onopen: ((ev: Event) => void) | null = null
	onerror: ((ev: Event) => void) | null = null
	onclose: ((ev: CloseEvent) => void) | null = null
	send = vi.fn()
	close = vi.fn()

	constructor(url: string) {
		this.url = url
		FakeWebSocket.instances.push(this)
	}

	simulateOpen() {
		this.readyState = FakeWebSocket.OPEN
		this.onopen?.(new Event("open"))
	}

	simulateMessage(data: string | ArrayBuffer) {
		// jsdom's MessageEvent stringifies ArrayBuffer data, so pass a plain object
		// carrying the buffer for the binary path (the client only reads ev.data).
		if (data instanceof ArrayBuffer) {
			this.onmessage?.({ data } as MessageEvent)
		} else {
			this.onmessage?.(new MessageEvent("message", { data }))
		}
	}

	simulateClose(code = 1000, reason = "") {
		this.readyState = FakeWebSocket.CLOSED
		this.onclose?.(new CloseEvent("close", { code, reason }))
	}

	simulateError() {
		this.onerror?.(new Event("error"))
	}
}

let clientSeq = 0

// Each client gets a unique port so tests can identify their own WebSocket
// instances even though focus listeners from earlier clients leak across tests.
function createClient(compress = false): WSClient {
	const seq = ++clientSeq
	return new WSClient(`ws://localhost:${9000 + seq}/ws`, compress)
}

function ownInstances(addr: string): FakeWebSocket[] {
	return FakeWebSocket.instances.filter(ws => ws.url.startsWith(`${addr}?`))
}

function lastOwnWS(addr: string): FakeWebSocket {
	return ownInstances(addr).at(-1)!
}

function toArrayBuffer(buf: Buffer): ArrayBuffer {
	// Node's pooled Buffer .buffer produces a cross-realm ArrayBuffer that fails
	// `instanceof ArrayBuffer` inside the module under test; allocate fresh instead.
	const ab = new ArrayBuffer(buf.byteLength)
	new Uint8Array(ab).set(buf)
	return ab
}

beforeEach(() => {
	vi.restoreAllMocks()
	vi.useFakeTimers()
	FakeWebSocket.instances = []
	vi.stubGlobal("WebSocket", FakeWebSocket)
	vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true }))
})

afterEach(() => {
	vi.useRealTimers()
	vi.unstubAllEnvs()
	vi.unstubAllGlobals()
})

describe("checkUpdate", () => {
	beforeEach(() => {
		localStorage.clear()
		document.head.innerHTML = ""
	})

	it("does nothing in dev mode", () => {
		// import.meta.env.PROD is false in vitest by default
		expect(() => checkUpdate("some-etag")).not.toThrow()
	})

	it("does nothing when etag is empty", () => {
		vi.stubEnv("PROD", true)
		expect(() => checkUpdate("")).not.toThrow()
	})

	it("logs when meta tag not found in head", () => {
		vi.stubEnv("PROD", true)
		const spy = vi.spyOn(console, "log").mockImplementation(() => {})
		checkUpdate("some-etag")
		expect(spy).toHaveBeenCalledWith("Not checking for update, frontend etag not found in head")
	})

	it("logs when etags match", () => {
		vi.stubEnv("PROD", true)
		const meta = document.createElement("meta")
		meta.name = "gomuks-frontend-etag"
		meta.content = "etag-123"
		document.head.appendChild(meta)
		const spy = vi.spyOn(console, "log").mockImplementation(() => {})
		checkUpdate("etag-123")
		expect(spy).toHaveBeenCalledWith("Frontend is up to date")
	})

	it("warns when etags mismatch but localStorage already attempted", () => {
		vi.stubEnv("PROD", true)
		const meta = document.createElement("meta")
		meta.name = "gomuks-frontend-etag"
		meta.content = "etag-new"
		document.head.appendChild(meta)
		localStorage.lastUpdateTo = "etag-server"
		const spy = vi.spyOn(console, "warn").mockImplementation(() => {})
		checkUpdate("etag-server")
		expect(spy).toHaveBeenCalled()
	})

	it("records attempted update when etags mismatch", () => {
		vi.stubEnv("PROD", true)
		const meta = document.createElement("meta")
		meta.name = "gomuks-frontend-etag"
		meta.content = "etag-new"
		document.head.appendChild(meta)
		// jsdom will log "Not implemented: navigation" but not throw
		const spy = vi.spyOn(console, "info").mockImplementation(() => {})
		checkUpdate("etag-server")
		expect(spy).toHaveBeenCalled()
		expect(localStorage.lastUpdateTo).toBe("etag-server")
	})
})

describe("WSClient", () => {
	let client: WSClient
	let addr: string

	beforeEach(() => {
		client = createClient()
		addr = (client as unknown as { addr: string }).addr
	})

	describe("constructor", () => {
		it("registers focus listener", () => {
			const spy = vi.spyOn(window, "addEventListener")
			new WSClient("ws://localhost:8080/ws")
			expect(spy).toHaveBeenCalledWith("focus", expect.any(Function))
		})

		it("defaults compress to false", () => {
			expect((client as unknown as { compress: boolean }).compress).toBe(false)
		})
	})

	describe("start", () => {
		it("creates WebSocket with correct URL", () => {
			client.start()
			const ws = lastOwnWS(addr)
			expect(ws.url.startsWith(`${addr}?`)).toBe(true)
			expect(ws.binaryType).toBe("arraybuffer")
		})

		it("includes resume params when reconnecting after run_id + events", () => {
			client.start()
			const ws = lastOwnWS(addr)
			ws.simulateOpen()
			// run_id sets resumeRunID and listenerID
			ws.simulateMessage(JSON.stringify({
				command: "run_id",
				request_id: 1,
				data: { run_id: "run-1", listener_id: 5, vapid_key: "vkey", etag: "etag1" },
			}))
			// a negative request_id sets lastReceivedEvt
			ws.simulateMessage(JSON.stringify({ command: "sync_status", request_id: -77, data: {} }))
			ws.simulateClose(1006, "lost")
			vi.advanceTimersByTime(20000)
			const ws2 = lastOwnWS(addr)
			expect(ws2.url).toContain("run_id=run-1")
			expect(ws2.url).toContain("last_received_event=-77")
			expect(ws2.url).toContain("prev_listener_id=5")
		})

		it("includes server timestamp when available", () => {
			client.getCachedServerTimestamp = () => 12345
			client.start()
			expect(lastOwnWS(addr).url).toContain("last_server_ts=12345")
		})

		it("includes compress param when compress enabled", () => {
			const compressClient = createClient(true)
			const compressAddr = (compressClient as unknown as { addr: string }).addr
			compressClient.start()
			expect(lastOwnWS(compressAddr).url).toContain("compress=1")
		})

		it("throws when already running", () => {
			client.start()
			expect(() => client.start()).toThrow("Tried to start new websocket while one is already running")
		})
	})

	describe("isConnected", () => {
		it("returns false when no connection", () => {
			expect(client.isConnected).toBe(false)
		})

		it("returns true when WebSocket is OPEN", () => {
			client.start()
			lastOwnWS(addr).simulateOpen()
			expect(client.isConnected).toBe(true)
		})
	})

	describe("send", () => {
		it("throws when no connection", () => {
			expect(() => client.send({ command: "test", request_id: 1, data: {} } as RPCCommand)).toThrow("Websocket not connected")
		})

		it("sends JSON stringified data", () => {
			client.start()
			const ws = lastOwnWS(addr)
			ws.simulateOpen()
			const data: RPCCommand = { command: "test", request_id: 1, data: { foo: "bar" } }
			client.send(data)
			expect(ws.send).toHaveBeenCalledWith(JSON.stringify(data))
		})
	})

	describe("onOpen", () => {
		it("dispatches connected status", () => {
			const listener = vi.fn()
			client.connect.listen(listener)
			client.start()
			lastOwnWS(addr).simulateOpen()
			expect(listener).toHaveBeenCalledWith(expect.objectContaining({
				connected: true,
				reconnecting: false,
				error: null,
			}))
		})

		it("resets connect failure count", () => {
			client.start()
			const ws = lastOwnWS(addr)
			ws.simulateOpen()
			ws.simulateClose(1006)
			// first failure backoff = 2^(1-4) * 1000 = 125ms
			vi.advanceTimersByTime(130)
			const ws2 = lastOwnWS(addr)
			ws2.simulateOpen()
			ws2.simulateClose(1006)
			// failure count was reset by successful open, so backoff is 125ms again
			vi.advanceTimersByTime(130)
			expect(ownInstances(addr).length).toBe(3)
		})

		it("sets up ping interval", () => {
			client.start()
			const ws = lastOwnWS(addr)
			ws.simulateOpen()
			vi.advanceTimersByTime(15000)
			expect(ws.send).toHaveBeenCalled()
		})
	})

	describe("pingLoop", () => {
		it("closes connection on timeout", () => {
			client.start()
			const ws = lastOwnWS(addr)
			ws.simulateOpen()
			// RECV_TIMEOUT = 60s; lastMessage set at open; interval ticks at 15/30/45/60/75s.
			// At 75s: Date.now() - lastMessage = 75s > 60s -> close
			vi.advanceTimersByTime(80000)
			expect(ws.close).toHaveBeenCalledWith(4002, "Ping timeout")
		})

		it("sends ping when within timeout", () => {
			client.start()
			const ws = lastOwnWS(addr)
			ws.simulateOpen()
			vi.advanceTimersByTime(15000)
			expect(ws.send).toHaveBeenCalledWith(expect.stringContaining('"command":"ping"'))
		})

		it("includes last received event id in ping", () => {
			client.start()
			const ws = lastOwnWS(addr)
			ws.simulateOpen()
			ws.simulateMessage(JSON.stringify({ command: "sync_status", request_id: -33, data: {} }))
			ws.send.mockClear()
			vi.advanceTimersByTime(15000)
			expect(ws.send).toHaveBeenCalledWith(expect.stringContaining('"last_received_id":-33'))
		})
	})

	describe("onMessage", () => {
		it("parses JSON message and dispatches event", () => {
			const listener = vi.fn()
			client.event.listen(listener)
			client.start()
			const ws = lastOwnWS(addr)
			ws.simulateOpen()
			ws.simulateMessage(JSON.stringify({
				command: "client_state",
				request_id: -1,
				data: { is_logged_in: true },
			}))
			expect(listener).toHaveBeenCalledWith(expect.objectContaining({ command: "client_state" }))
		})

		it("closes on malformed JSON", () => {
			client.start()
			const ws = lastOwnWS(addr)
			ws.simulateOpen()
			ws.simulateMessage("not json")
			expect(ws.close).toHaveBeenCalledWith(1003, "Malformed JSON")
		})

		it("closes on missing command field", () => {
			client.start()
			const ws = lastOwnWS(addr)
			ws.simulateOpen()
			ws.simulateMessage(JSON.stringify({ request_id: 1, data: {} }))
			expect(ws.close).toHaveBeenCalledWith(1003, "Malformed JSON")
		})

		it("updates lastReceivedEvt for negative request_id", () => {
			client.start()
			const ws = lastOwnWS(addr)
			ws.simulateOpen()
			ws.simulateMessage(JSON.stringify({ command: "run_id", request_id: 1, data: { run_id: "run-x" } }))
			ws.simulateMessage(JSON.stringify({ command: "sync_status", request_id: -100, data: {} }))
			ws.simulateClose(1006)
			vi.advanceTimersByTime(20000)
			const ws2 = lastOwnWS(addr)
			expect(ws2.url).toContain("last_received_event=-100")
		})

		it("handles run_id command", () => {
			client.start()
			const ws = lastOwnWS(addr)
			ws.simulateOpen()
			ws.simulateMessage(JSON.stringify({
				command: "run_id",
				request_id: 1,
				data: { run_id: "run-abc", listener_id: 42, vapid_key: "vkey123", etag: "etag-xyz" },
			}))
			expect((window as unknown as { vapidPublicKey?: string }).vapidPublicKey).toBe("vkey123")
		})
	})

	describe("compressed messages", () => {
		it("decompresses ArrayBuffer messages and dispatches events", async () => {
			const compressClient = createClient(true)
			const compressAddr = (compressClient as unknown as { addr: string }).addr
			const listener = vi.fn()
			compressClient.event.listen(listener)
			compressClient.start()
			const ws = lastOwnWS(compressAddr)
			ws.simulateOpen()
			const payload = JSON.stringify({ command: "sync_status", request_id: -5, data: { type: "ok" } })
			ws.simulateMessage(toArrayBuffer(zlib.deflateRawSync(payload)))
			await vi.waitFor(() => expect(listener).toHaveBeenCalled())
			expect(listener).toHaveBeenCalledWith(expect.objectContaining({ command: "sync_status" }))
		})

		it("closes on malformed JSON object in decompression stream", async () => {
			const compressClient = createClient(true)
			const compressAddr = (compressClient as unknown as { addr: string }).addr
			compressClient.start()
			const ws = lastOwnWS(compressAddr)
			ws.simulateOpen()
			const payload = JSON.stringify({ foo: "no command field" })
			ws.simulateMessage(toArrayBuffer(zlib.deflateRawSync(payload)))
			await vi.waitFor(() => expect(ws.close).toHaveBeenCalledWith(1003, "Malformed JSON in decompression stream"))
		})
	})

	describe("onClose", () => {
		it("rejects pending requests", async () => {
			client.start()
			const ws = lastOwnWS(addr)
			ws.simulateOpen()
			const promise = client.request("test", {})
			ws.simulateClose(1006, "lost")
			await expect(promise).rejects.toThrow("Websocket closed")
		})

		it("clears ping interval", () => {
			client.start()
			const ws = lastOwnWS(addr)
			ws.simulateOpen()
			ws.simulateClose(1000)
			ws.send.mockClear()
			vi.advanceTimersByTime(15000)
			expect(ws.send).not.toHaveBeenCalled()
		})

		it("triggers reconnect with backoff", () => {
			const listener = vi.fn()
			client.connect.listen(listener)
			client.start()
			const ws = lastOwnWS(addr)
			ws.simulateOpen()
			ws.simulateClose(1006, "lost")
			expect(listener).toHaveBeenCalledWith(expect.objectContaining({
				reconnecting: true,
				nextAttempt: expect.any(String),
			}))
			vi.advanceTimersByTime(200)
			expect(ownInstances(addr).length).toBe(2)
		})
	})

	describe("stop", () => {
		it("closes connection and clears intervals", () => {
			client.start()
			const ws = lastOwnWS(addr)
			ws.simulateOpen()
			client.stop()
			expect(ws.close).toHaveBeenCalledWith(1000, "Client closed")
		})

		it("prevents reconnect after close", () => {
			client.start()
			const ws = lastOwnWS(addr)
			ws.simulateOpen()
			client.stop()
			ws.simulateClose(1000)
			vi.advanceTimersByTime(20000)
			expect(ownInstances(addr).length).toBe(1)
		})
	})

	describe("checkAuthAndStart", () => {
		it("calls start on auth success", async () => {
			const startSpy = vi.spyOn(client, "start")
			client.checkAuthAndStart()
			await vi.runAllTimersAsync()
			expect(startSpy).toHaveBeenCalled()
		})

		it("dispatches error on auth failure", async () => {
			vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
				ok: false,
				status: 401,
				statusText: "Unauthorized",
				text: () => Promise.resolve("bad creds"),
			}))
			const listener = vi.fn()
			client.connect.listen(listener)
			client.checkAuthAndStart()
			await vi.runAllTimersAsync()
			expect(listener).toHaveBeenCalledWith(expect.objectContaining({
				connected: false,
				reconnecting: false,
				error: expect.stringContaining("Authentication failed"),
			}))
		})

		it("reconnects on non-auth error", async () => {
			vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("network down")))
			const listener = vi.fn()
			client.connect.listen(listener)
			client.checkAuthAndStart()
			await vi.runAllTimersAsync()
			expect(listener).toHaveBeenCalledWith(expect.objectContaining({
				reconnecting: true,
			}))
		})

		it("propagates previous connection error while trying again", async () => {
			const listener = vi.fn()
			client.connect.listen(listener)
			client.connect.emit({ connected: false, reconnecting: false, error: "old error" })
			client.checkAuthAndStart()
			await vi.runAllTimersAsync()
			expect(listener).toHaveBeenCalledWith(expect.objectContaining({
				reconnecting: true,
				error: "old error",
				nextAttempt: "currently trying to connect",
			}))
		})
	})

	describe("onFocus", () => {
		it("reconnects immediately when reconnect timeout pending", async () => {
			client.start()
			const ws = lastOwnWS(addr)
			ws.simulateOpen()
			ws.simulateClose(1006)
			window.dispatchEvent(new Event("focus"))
			await vi.runAllTimersAsync()
			// focus triggers checkAuthAndStart -> doAuth -> start
			expect(ownInstances(addr).length).toBeGreaterThanOrEqual(2)
		})
	})

	describe("onError", () => {
		it("logs the error", () => {
			const spy = vi.spyOn(console, "error").mockImplementation(() => {})
			client.start()
			const ws = lastOwnWS(addr)
			ws.simulateOpen()
			ws.simulateError()
			expect(spy).toHaveBeenCalledWith("Websocket error:", expect.anything())
		})
	})
})
