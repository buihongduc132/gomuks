import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"

// Mock the worker import before importing WasmClient
vi.mock("./wasm/wasmuks.ts?worker", () => {
	return { default: FakeWasmWorker }
})

class FakeWasmWorker {
	static instances: FakeWasmWorker[] = []
	name?: string
	onmessage: ((ev: MessageEvent) => void) | null = null
	addEventListener = vi.fn((event: string, handler: (ev: MessageEvent) => void) => {
		if (event === "message") this.onmessage = handler
	})
	postMessage = vi.fn()
	terminate = vi.fn()

	constructor(opts?: { name?: string }) {
		this.name = opts?.name
		FakeWasmWorker.instances.push(this)
	}

	simulateMessage(data: unknown) {
		this.onmessage?.(new MessageEvent("message", { data }))
	}
}

// Mock navigator.storage and serviceWorker
beforeEach(() => {
	vi.restoreAllMocks()
	FakeWasmWorker.instances = []
	const mockPersist = vi.fn().mockResolvedValue("granted")
	const mockRegister = vi.fn().mockResolvedValue({ active: { scriptURL: "sw.js" } })
	vi.stubGlobal("navigator", {
		...navigator,
		storage: { persist: mockPersist },
		serviceWorker: { register: mockRegister },
	})
})

afterEach(() => {
	vi.unstubAllGlobals()
})

async function createClient() {
	const { default: WasmClient } = await import("./wasmclient")
	return new WasmClient()
}

function getLastWorker(): FakeWasmWorker {
	return FakeWasmWorker.instances.at(-1)!
}

describe("WasmClient", () => {
	describe("start", () => {
		it("creates worker with correct name", async () => {
			const client = await createClient()
			await client.start()
			expect(getLastWorker().name).toBe("gomuks-wasm-worker")
		})

		it("registers message listener on worker", async () => {
			const client = await createClient()
			await client.start()
			expect(getLastWorker().addEventListener).toHaveBeenCalledWith("message", expect.any(Function))
		})

		it("requests storage persistence", async () => {
			const client = await createClient()
			await client.start()
			expect(navigator.storage.persist).toHaveBeenCalled()
		})

		it("registers media service worker", async () => {
			const client = await createClient()
			await client.start()
			expect(navigator.serviceWorker.register).toHaveBeenCalledWith("wasmuks-media-sw.js")
		})
	})

	describe("doAuth", () => {
		it("resolves without doing anything", async () => {
			const client = await createClient()
			await expect(client.doAuth()).resolves.toBeUndefined()
		})
	})

	describe("isConnected", () => {
		it("is always true", async () => {
			const client = await createClient()
			expect(client.isConnected).toBe(true)
		})
	})

	describe("rpcMediaUpload", () => {
		it("is true", async () => {
			const client = await createClient()
			expect(client.rpcMediaUpload).toBe(true)
		})
	})

	describe("uploadMedia", () => {
		it("rejects when worker not initialized", async () => {
			const client = await createClient()
			const blob = new Blob(["test"], { type: "text/plain" })
			await expect(client.uploadMedia(blob, "test.txt", false)).rejects.toThrow("Worker not initialized")
		})

		it("posts message to worker with payload and resolves response", async () => {
			const client = await createClient()
			await client.start()
			const worker = getLastWorker()
			
			const fakeBytes = new Uint8Array([116, 101, 115, 116])
			const fakeBlob = {
				bytes: () => Promise.resolve(fakeBytes),
			} as unknown as Blob
			
			const promise = client.uploadMedia(fakeBlob, "test.txt", true)
			
			await Promise.resolve()
			await Promise.resolve()
			await Promise.resolve()
			
			expect(worker.postMessage).toHaveBeenCalledTimes(1)
			expect(worker.postMessage).toHaveBeenCalledWith(
				expect.objectContaining({
					command: "wasm-upload",
					filename: "test.txt",
					encrypt: true,
				}),
				expect.any(Array),
			)
			// Simulate worker response — wasm worker protocol sends data as JSON string
			const request_id = worker.postMessage.mock.calls[0][0].request_id
			worker.simulateMessage({
				command: "response",
				request_id,
				data: JSON.stringify({ url: "mxc://example.com/media123" }),
			})
			await expect(promise).resolves.toEqual({ url: "mxc://example.com/media123" })
		})
	})

	describe("onMessage", () => {
		it("parses JSON string data", async () => {
			const client = await createClient()
			await client.start()
			const listener = vi.fn()
			client.event.listen(listener)
			const worker = getLastWorker()
			worker.simulateMessage({
				command: "sync_status",
				request_id: -1,
				data: JSON.stringify({ type: "ok" }),
			})
			expect(listener).toHaveBeenCalledWith(expect.objectContaining({ command: "sync_status" }))
		})

		it("handles wasm-connection command", async () => {
			const client = await createClient()
			await client.start()
			const listener = vi.fn()
			client.connect.listen(listener)
			const worker = getLastWorker()
			worker.simulateMessage({
				command: "wasm-connection",
				request_id: 0,
				data: { connected: true, reconnecting: false, error: null },
			})
			expect(listener).toHaveBeenCalledWith(expect.objectContaining({ connected: true }))
		})

		it("logs error for unexpected message data", async () => {
			const spy = vi.spyOn(console, "error").mockImplementation(() => {})
			const client = await createClient()
			await client.start()
			const worker = getLastWorker()
			worker.simulateMessage({
				command: "some_cmd",
				request_id: 1,
				data: 12345, // not a string, not wasm-connection
			})
			expect(spy).toHaveBeenCalledWith("Unexpected message data:", expect.anything())
		})

		it("dispatches response commands to pending requests", async () => {
			const client = await createClient()
			await client.start()
			const worker = getLastWorker()
			// Make a request
			const promise = client.request("test_cmd", { foo: "bar" })
			const request_id = worker.postMessage.mock.calls[0][0].request_id
			worker.simulateMessage({
				command: "response",
				request_id,
				data: JSON.stringify({ result: "ok" }),
			})
			await expect(promise).resolves.toEqual({ result: "ok" })
		})
	})

	describe("send", () => {
		it("throws when worker not initialized", async () => {
			const client = await createClient()
			expect(() => client.send({ command: "test", request_id: 1, data: {} })).toThrow("Worker not initialized")
		})

		it("posts JSON stringified data to worker", async () => {
			const client = await createClient()
			await client.start()
			const worker = getLastWorker()
			client.send({ command: "test_cmd", request_id: 42, data: { foo: "bar" } })
			expect(worker.postMessage).toHaveBeenCalledWith({
				command: "test_cmd",
				request_id: 42,
				data: JSON.stringify({ foo: "bar" }),
			})
		})

		it("uses empty string for missing command", async () => {
			const client = await createClient()
			await client.start()
			const worker = getLastWorker()
			client.send({ request_id: 1, data: {} } as any)
			expect(worker.postMessage).toHaveBeenCalledWith(
				expect.objectContaining({ command: "" }),
			)
		})

		it("uses 0 for missing request_id", async () => {
			const client = await createClient()
			await client.start()
			const worker = getLastWorker()
			client.send({ command: "test", data: {} } as any)
			expect(worker.postMessage).toHaveBeenCalledWith(
				expect.objectContaining({ request_id: 0 }),
			)
		})

		it("uses empty object for missing data", async () => {
			const client = await createClient()
			await client.start()
			const worker = getLastWorker()
			client.send({ command: "test", request_id: 1 } as any)
			expect(worker.postMessage).toHaveBeenCalledWith(
				expect.objectContaining({ data: "{}" }),
			)
		})
	})

	describe("stop", () => {
		it("terminates worker", async () => {
			const client = await createClient()
			await client.start()
			const worker = getLastWorker()
			await client.stop()
			expect(worker.terminate).toHaveBeenCalled()
		})

		it("can be called when worker not initialized", async () => {
			const client = await createClient()
			await expect(client.stop()).resolves.toBeUndefined()
		})
	})
})
