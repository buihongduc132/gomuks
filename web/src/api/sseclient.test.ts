import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import SSEClient from "./sseclient"

// Mock EventSource
class FakeEventSource {
	static instances: FakeEventSource[] = []
	static CONNECTING = 0
	static OPEN = 1
	static CLOSED = 2

	readyState = FakeEventSource.CONNECTING
	url: string
	onmessage: ((ev: MessageEvent) => void) | null = null
	onopen: ((ev: Event) => void) | null = null
	onerror: ((ev: Event) => void) | null = null
	close = vi.fn()

	constructor(url: string) {
		this.url = url
		FakeEventSource.instances.push(this)
	}

	simulateOpen() {
		this.readyState = FakeEventSource.OPEN
		this.onopen?.(new Event("open"))
	}

	simulateMessage(data: string) {
		this.onmessage?.(new MessageEvent("message", { data }))
	}

	simulateError() {
		this.onerror?.(new Event("error"))
	}
}

beforeEach(() => {
	vi.restoreAllMocks()
	vi.useFakeTimers()
	FakeEventSource.instances = []
	vi.stubGlobal("EventSource", FakeEventSource)
	vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({}) }))
})

afterEach(() => {
	vi.useRealTimers()
	vi.unstubAllGlobals()
})

function getLastES(): FakeEventSource {
	return FakeEventSource.instances.at(-1)!
}

describe("SSEClient", () => {
	let client: SSEClient

	beforeEach(() => {
		client = new SSEClient()
	})

	describe("start", () => {
		it("registers focus listener on start", () => {
			const spy = vi.spyOn(window, "addEventListener")
			client.start()
			expect(spy).toHaveBeenCalledWith("focus", expect.any(Function))
		})

		it("creates EventSource with SSE URL", () => {
			client.start()
			expect(getLastES().url.startsWith("_gomuks/sse?")).toBe(true)
		})

		it("dispatches reconnecting status while connecting", () => {
			const listener = vi.fn()
			client.connect.listen(listener)
			client.start()
			expect(listener).toHaveBeenCalledWith(expect.objectContaining({
				connected: false,
				reconnecting: true,
				nextAttempt: "currently trying to connect",
			}))
		})

		it("includes resume params when reconnecting after run_id + events", () => {
			client.start()
			const es = getLastES()
			es.simulateMessage(JSON.stringify({
				command: "run_id",
				request_id: 1,
				data: { run_id: "run-1", listener_id: 7, vapid_key: "vk", etag: "e1" },
			}))
			es.simulateMessage(JSON.stringify({ command: "sync_status", request_id: -42, data: {} }))
			es.simulateError()
			// backoff for first failure = 2^(1-4) * 1000 = 125ms
			vi.advanceTimersByTime(130)
			const es2 = getLastES()
			expect(es2.url).toContain("run_id=run-1")
			expect(es2.url).toContain("last_received_event=-42")
			expect(es2.url).toContain("prev_listener_id=7")
		})

		it("includes server timestamp when available", () => {
			client.getCachedServerTimestamp = () => 9999
			client.start()
			expect(getLastES().url).toContain("last_server_ts=9999")
		})
	})

	describe("isConnected", () => {
		it("returns false when no connection", () => {
			expect(client.isConnected).toBe(false)
		})

		it("returns true when EventSource is OPEN", () => {
			client.start()
			getLastES().simulateOpen()
			expect(client.isConnected).toBe(true)
		})
	})

	describe("send", () => {
		it("throws", () => {
			expect(() => client.send()).toThrow("Raw sends aren't supported with SSE")
		})
	})

	describe("onOpen", () => {
		it("dispatches connected status", () => {
			const listener = vi.fn()
			client.connect.listen(listener)
			client.start()
			getLastES().simulateOpen()
			expect(listener).toHaveBeenCalledWith(expect.objectContaining({
				connected: true,
				reconnecting: false,
				error: null,
			}))
		})

		it("sets up ping interval", () => {
			client.start()
			const es = getLastES()
			es.simulateOpen()
			// set up ping prerequisites
			es.simulateMessage(JSON.stringify({
				command: "run_id",
				request_id: 1,
				data: { run_id: "run-1", listener_id: 7 },
			}))
			es.simulateMessage(JSON.stringify({ command: "sync_status", request_id: -42, data: {} }))
			;(fetch as ReturnType<typeof vi.fn>).mockResolvedValue({ ok: true, json: () => Promise.resolve({}) })
			vi.advanceTimersByTime(60000)
			expect(fetch).toHaveBeenCalledWith(
				expect.stringContaining("_gomuks/sse/ping?"),
				expect.objectContaining({ method: "POST" }),
			)
		})
	})

	describe("onMessage", () => {
		it("emits parsed events", () => {
			const listener = vi.fn()
			client.event.listen(listener)
			client.start()
			const es = getLastES()
			es.simulateOpen()
			es.simulateMessage(JSON.stringify({ command: "sync_status", request_id: -1, data: { type: "ok" } }))
			expect(listener).toHaveBeenCalledWith(expect.objectContaining({ command: "sync_status" }))
		})

		it("handles run_id command and sets vapid key", () => {
			client.start()
			const es = getLastES()
			es.simulateOpen()
			es.simulateMessage(JSON.stringify({
				command: "run_id",
				request_id: 1,
				data: { run_id: "run-x", listener_id: 3, vapid_key: "vkey-xyz", etag: "etag-abc" },
			}))
			expect((window as unknown as { vapidPublicKey?: string }).vapidPublicKey).toBe("vkey-xyz")
		})

		it("does not track lastReceivedEvt for positive request_id", () => {
			client.start()
			const es = getLastES()
			es.simulateOpen()
			es.simulateMessage(JSON.stringify({ command: "sync_status", request_id: 5, data: {} }))
			es.simulateError()
			vi.advanceTimersByTime(130)
			const es2 = getLastES()
			expect(es2.url).not.toContain("last_received_event=")
		})
	})

	describe("onError", () => {
		it("dispatches disconnected status with backoff", () => {
			const listener = vi.fn()
			client.connect.listen(listener)
			client.start()
			getLastES().simulateError()
			expect(listener).toHaveBeenCalledWith(expect.objectContaining({
				connected: false,
				reconnecting: true,
				error: "SSE disconnected",
				nextAttempt: expect.any(String),
			}))
		})

		it("closes connection and reconnects after backoff", () => {
			client.start()
			const es = getLastES()
			es.simulateOpen()
			es.simulateError()
			expect(es.close).toHaveBeenCalled()
			vi.advanceTimersByTime(130)
			expect(FakeEventSource.instances.length).toBe(2)
		})

		it("does not reconnect when stopped", () => {
			client.start()
			const es = getLastES()
			es.simulateOpen()
			client.stop()
			es.simulateError()
			vi.advanceTimersByTime(20000)
			expect(FakeEventSource.instances.length).toBe(1)
		})
	})

	describe("pingLoop", () => {
		it("skips ping when prerequisites missing", () => {
			client.start()
			const es = getLastES()
			es.simulateOpen()
			// no run_id received -> no resumeRunID
			vi.advanceTimersByTime(120000)
			expect(fetch).not.toHaveBeenCalledWith(
				expect.stringContaining("_gomuks/sse/ping"),
				expect.anything(),
			)
		})

		it("sends ping and acks event", async () => {
			client.start()
			const es = getLastES()
			es.simulateOpen()
			es.simulateMessage(JSON.stringify({
				command: "run_id",
				request_id: 1,
				data: { run_id: "run-1", listener_id: 7 },
			}))
			es.simulateMessage(JSON.stringify({ command: "sync_status", request_id: -42, data: {} }))
			;(fetch as ReturnType<typeof vi.fn>).mockResolvedValue({ ok: true, json: () => Promise.resolve({}) })
			vi.advanceTimersByTime(60000)
			// Let the fetch promise resolve
			await Promise.resolve()
			await Promise.resolve()
			expect(fetch).toHaveBeenCalledWith(
				expect.stringContaining("run_id=run-1"),
				expect.objectContaining({ method: "POST" }),
			)
			// second ping interval should NOT re-send same event (acked)
			;(fetch as ReturnType<typeof vi.fn>).mockClear()
			vi.advanceTimersByTime(60000)
			expect(fetch).not.toHaveBeenCalled()
		})

		it("logs error when ping fails", async () => {
			const errSpy = vi.spyOn(console, "error").mockImplementation(() => {})
			client.start()
			const es = getLastES()
			es.simulateOpen()
			es.simulateMessage(JSON.stringify({
				command: "run_id",
				request_id: 1,
				data: { run_id: "run-1", listener_id: 7 },
			}))
			es.simulateMessage(JSON.stringify({ command: "sync_status", request_id: -42, data: {} }))
			;(fetch as ReturnType<typeof vi.fn>).mockRejectedValue(new Error("network down"))
			vi.advanceTimersByTime(60000)
			await Promise.resolve()
			await Promise.resolve()
			expect(errSpy).toHaveBeenCalledWith("Failed to send ping:", expect.any(Error))
			// not acked -> next interval retries
			vi.advanceTimersByTime(60000)
			expect(fetch).toHaveBeenCalledTimes(2)
		})
	})

	describe("request", () => {
		it("resolves with JSON payload on ok response", async () => {
			;(fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
				ok: true,
				status: 200,
				json: () => Promise.resolve({ result: "ok" }),
			})
			await expect(client.request("test_cmd", { foo: 1 })).resolves.toEqual({ result: "ok" })
			expect(fetch).toHaveBeenCalledWith("_gomuks/exec/test_cmd", {
				method: "POST",
				body: JSON.stringify({ foo: 1 }),
				headers: { "Content-Type": "application/json" },
			})
		})

		it("rejects with error string from payload on error response", async () => {
			;(fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
				ok: false,
				status: 400,
				json: () => Promise.resolve({ error: "bad input" }),
			})
			await expect(client.request("test_cmd", {})).rejects.toThrow("bad input")
		})

		it("rejects with unexpected JSON message on error without error string", async () => {
			;(fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
				ok: false,
				status: 500,
				json: () => Promise.resolve({ other: "data" }),
			})
			await expect(client.request("test_cmd", {})).rejects.toThrow("Unexpected JSON response with status 500")
		})

		it("rejects on non-JSON response", async () => {
			;(fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
				ok: true,
				status: 200,
				json: () => Promise.reject(new Error("not json")),
			})
			await expect(client.request("test_cmd", {})).rejects.toThrow("Non-JSON response with status 200")
		})

		it("rejects on network failure", async () => {
			;(fetch as ReturnType<typeof vi.fn>).mockRejectedValue(new Error("network down"))
			await expect(client.request("test_cmd", {})).rejects.toThrow("network down")
		})
	})

	describe("stop", () => {
		it("closes EventSource and clears intervals", () => {
			client.start()
			const es = getLastES()
			es.simulateOpen()
			client.stop()
			expect(es.close).toHaveBeenCalled()
		})

		it("can be restarted after stop", () => {
			client.start()
			client.stop()
			client.start()
			expect(FakeEventSource.instances.length).toBe(2)
		})
	})

	describe("onFocus", () => {
		it("reconnects immediately when reconnect timeout pending", () => {
			client.start()
			const es = getLastES()
			es.simulateError()
			const countBefore = FakeEventSource.instances.length
			window.dispatchEvent(new Event("focus"))
			vi.advanceTimersByTime(10)
			expect(FakeEventSource.instances.length).toBeGreaterThan(countBefore)
		})
	})
})
