import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import RPCClient, { ErrorResponse } from "./rpc"
import type { RPCCommand, RPCEvent } from "./types"

// Concrete subclass for testing
class TestRPC extends RPCClient {
	protected isConnected = true
	protected send = vi.fn()
	start = vi.fn()
	stop = vi.fn()

	// Expose protected methods for testing
	callOnCommand(data: RPCCommand) {
		this.onCommand(data)
	}
	callCancelRequest(request_id: number, reason: string) {
		this.cancelRequest(request_id, reason)
	}
	getNextRequestID() {
		return this.nextRequestID
	}
	setConnected(val: boolean) {
		this.isConnected = val
	}
	getPendingCount() {
		return this.pendingRequests.size
	}
}

beforeEach(() => {
	vi.restoreAllMocks()
})

describe("ErrorResponse", () => {
	it("creates error with data", () => {
		const err = new ErrorResponse("bad data")
		expect(err.data).toBe("bad data")
		expect(err.message).toBe("bad data")
		expect(err instanceof Error).toBe(true)
	})
})

describe("RPCClient", () => {
	let rpc: TestRPC

	beforeEach(() => {
		rpc = new TestRPC()
	})

	describe("onCommand", () => {
		it("resolves pending request on response", async () => {
			const promise = rpc.request("test", {})
			const request_id = rpc.getNextRequestID() - 1
			rpc.callOnCommand({ command: "response", request_id, data: { result: "ok" } } as RPCCommand)
			await expect(promise).resolves.toEqual({ result: "ok" })
		})

		it("rejects pending request on error", async () => {
			const promise = rpc.request("test", {})
			const request_id = rpc.getNextRequestID() - 1
			rpc.callOnCommand({ command: "error", request_id, data: "fail" } as RPCCommand)
			await expect(promise).rejects.toBeInstanceOf(ErrorResponse)
		})

		it("logs error for unknown request_id on response", () => {
			const spy = vi.spyOn(console, "error").mockImplementation(() => {})
			rpc.callOnCommand({ command: "response", request_id: 9999, data: {} } as RPCCommand)
			expect(spy).toHaveBeenCalledWith("Received response for unknown request:", expect.anything())
		})

		it("logs error for unknown request_id on error", () => {
			const spy = vi.spyOn(console, "error").mockImplementation(() => {})
			rpc.callOnCommand({ command: "error", request_id: 9999, data: {} } as RPCCommand)
			expect(spy).toHaveBeenCalled()
		})

		it("emits event for non-response commands", () => {
			const listener = vi.fn()
			rpc.event.listen(listener)
			rpc.callOnCommand({ command: "client_state", request_id: -1, data: { is_logged_in: true } } as RPCEvent)
			expect(listener).toHaveBeenCalledWith(expect.objectContaining({ command: "client_state" }))
		})
	})

	describe("cancelRequest", () => {
		it("logs debug for unknown request", () => {
			const spy = vi.spyOn(console, "debug").mockImplementation(() => {})
			rpc.callCancelRequest(9999, "test reason")
			expect(spy).toHaveBeenCalledWith("Tried to cancel unknown request", 9999)
		})

		it("sends cancel command for known request", () => {
			const debugSpy = vi.spyOn(console, "debug").mockImplementation(() => {})
			rpc.request("test", {})
			const request_id = rpc.getNextRequestID() - 1
			rpc.callCancelRequest(request_id, "test reason")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				command: "cancel",
				data: { request_id, reason: "test reason" },
			}))
			// Suppress unused
			void debugSpy
		})
	})

	describe("nextRequestID", () => {
		it("increments on each call", () => {
			const id1 = rpc.getNextRequestID()
			const id2 = rpc.getNextRequestID()
			expect(id2).toBe(id1 + 1)
		})
	})

	describe("uploadMedia", () => {
		it("throws by default", async () => {
			await expect(rpc.uploadMedia(new Blob(), "test.png", false)).rejects.toThrow("Media upload not supported")
		})
	})

	describe("doAuth", () => {
		it("succeeds on ok response", async () => {
			vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true }))
			await expect(rpc.doAuth()).resolves.toBeUndefined()
			expect(fetch).toHaveBeenCalledWith(expect.stringContaining("_gomuks/auth?secure="), expect.objectContaining({ method: "POST" }))
		})

		it("throws on non-ok response with body", async () => {
			vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
				ok: false,
				status: 401,
				statusText: "Unauthorized",
				text: () => Promise.resolve("bad creds"),
			}))
			await expect(rpc.doAuth()).rejects.toThrow("Authentication failed: 401 Unauthorized - bad creds")
		})

		it("throws on non-ok response without body", async () => {
			vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
				ok: false,
				status: 500,
				statusText: "Internal Server Error",
				text: () => Promise.resolve(""),
			}))
			await expect(rpc.doAuth()).rejects.toThrow("Authentication failed: 500 Internal Server Error")
		})

		it("throws on non-ok response when text() throws", async () => {
			vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
				ok: false,
				status: 500,
				statusText: "Error",
				text: () => Promise.reject(new Error("no text")),
			}))
			await expect(rpc.doAuth()).rejects.toThrow("Authentication failed: 500 Error")
		})
	})

	describe("tryAuth", () => {
		it("returns true on success", async () => {
			vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true }))
			const signal = new AbortController().signal
			await expect(rpc.tryAuth(signal)).resolves.toBe(true)
		})

		it("returns false and emits error on failure (not aborted)", async () => {
			vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
				ok: false,
				status: 401,
				statusText: "Unauthorized",
				text: () => Promise.resolve("bad"),
			}))
			const listener = vi.fn()
			rpc.connect.listen(listener)
			const ac = new AbortController()
			await expect(rpc.tryAuth(ac.signal)).resolves.toBe(false)
			expect(listener).toHaveBeenCalledWith(expect.objectContaining({ connected: false, reconnecting: false }))
		})

		it("returns false silently when aborted", async () => {
			vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
				ok: false,
				status: 401,
				statusText: "Unauthorized",
				text: () => Promise.resolve("bad"),
			}))
			const listener = vi.fn()
			rpc.connect.listen(listener)
			const ac = new AbortController()
			ac.abort()
			await expect(rpc.tryAuth(ac.signal)).resolves.toBe(false)
			expect(listener).not.toHaveBeenCalled()
		})
	})

	describe("request", () => {
		it("rejects immediately when not connected", async () => {
			rpc.setConnected(false)
			await expect(rpc.request("test", {})).rejects.toThrow("Websocket not connected")
		})

		it("sends command when connected", () => {
			rpc.request("test_cmd", { foo: "bar" })
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				command: "test_cmd",
				data: { foo: "bar" },
			}))
		})

		it("rejects if disconnected between check and send", async () => {
			// isConnected is checked once before creating the promise and again
			// inside the synchronous executor; flip the value in between via a getter
			let accesses = 0
			Object.defineProperty(rpc, "isConnected", {
				get: () => ++accesses <= 1,
				configurable: true,
			})
			try {
				await expect(rpc.request("test", {})).rejects.toThrow("Websocket not connected")
			} finally {
				delete (rpc as unknown as Record<string, unknown>).isConnected
			}
		})
	})

	describe("wrapper methods", () => {
		it("logout sends correct command", () => {
			rpc.logout()
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "logout" }))
		})

		it("sendMessage sends correct command", () => {
			rpc.sendMessage({ room_id: "!r:ex.com", text: "hello" })
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "send_message" }))
		})

		it("sendEvent sends correct command", () => {
			rpc.sendEvent("!r:ex.com", "m.room.message" as any, { body: "hi" })
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				command: "send_event",
				data: expect.objectContaining({ room_id: "!r:ex.com" }),
			}))
		})

		it("sendEvent passes disable_encryption and synchronous", () => {
			rpc.sendEvent("!r:ex.com", "m.room.message" as any, {}, true, true)
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				data: expect.objectContaining({ disable_encryption: true, synchronous: true }),
			}))
		})

		it("sendStickyEvent sends correct command", () => {
			rpc.sendStickyEvent("!r:ex.com", "m.room.message" as any, {}, 5000, 100)
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				command: "send_sticky_event",
				data: expect.objectContaining({ sticky_duration_ms: 5000, delay_ms: 100 }),
			}))
		})

		it("resendEvent sends correct command", () => {
			rpc.resendEvent("txn-1")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				command: "resend_event",
				data: { transaction_id: "txn-1" },
			}))
		})

		it("reportEvent sends correct command", () => {
			rpc.reportEvent("!r:ex.com", "$evt", "spam")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				command: "report_event",
				data: { room_id: "!r:ex.com", event_id: "$evt", reason: "spam" },
			}))
		})

		it("redactEvent sends correct command", () => {
			rpc.redactEvent("!r:ex.com", "$evt", "reason")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				command: "redact_event",
			}))
		})

		it("setState sends correct command with extra", () => {
			rpc.setState("!r:ex.com", "m.room.topic" as any, "", { topic: "hi" }, { delay_ms: 100 })
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				command: "set_state",
				data: expect.objectContaining({ delay_ms: 100 }),
			}))
		})

		it("updateDelayedEvent sends correct command", () => {
			rpc.updateDelayedEvent("delay-1", "cancel")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				command: "update_delayed_event",
			}))
		})

		it("setMembership sends correct command", () => {
			rpc.setMembership("!r:ex.com", "@u:ex.com", "ban", "spam", true)
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				command: "set_membership",
				data: expect.objectContaining({ action: "ban", msc4293_redact_events: true }),
			}))
		})

		it("setAccountData sends correct command", () => {
			rpc.setAccountData("m.some_type" as any, { key: "val" }, "!r:ex.com")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				command: "set_account_data",
				data: expect.objectContaining({ room_id: "!r:ex.com" }),
			}))
		})

		it("markRead sends correct command with default receipt type", () => {
			rpc.markRead("!r:ex.com", "$evt")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				command: "mark_read",
				data: expect.objectContaining({ receipt_type: "m.read" }),
			}))
		})

		it("markRead sends custom receipt type", () => {
			rpc.markRead("!r:ex.com", "$evt", "m.read.private")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				data: expect.objectContaining({ receipt_type: "m.read.private" }),
			}))
		})

		it("setTyping sends correct command", () => {
			rpc.setTyping("!r:ex.com", 5000)
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				command: "set_typing",
			}))
		})

		it("getProfile sends correct command", () => {
			rpc.getProfile("@u:ex.com")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "get_profile" }))
		})

		it("setProfileField sends correct command", () => {
			rpc.setProfileField("displayname", "Test")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "set_profile_field" }))
		})

		it("getMutualRooms sends correct command", () => {
			rpc.getMutualRooms("@u:ex.com")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "get_mutual_rooms" }))
		})

		it("getProfileEncryptionInfo sends correct command", () => {
			rpc.getProfileEncryptionInfo("@u:ex.com")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "get_profile_encryption_info" }))
		})

		it("getOwnDevices sends correct command", () => {
			rpc.getOwnDevices()
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "get_own_devices" }))
		})

		it("trackUserDevices sends correct command", () => {
			rpc.trackUserDevices("@u:ex.com")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "track_user_devices" }))
		})

		it("resetMasterKeyTOFU sends correct command", () => {
			rpc.resetMasterKeyTOFU("@u:ex.com", "key")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "reset_master_key_tofu" }))
		})

		it("ensureGroupSessionShared sends correct command", () => {
			rpc.ensureGroupSessionShared("!r:ex.com")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "ensure_group_session_shared" }))
		})

		it("sendToDevice sends correct command with default encrypted", () => {
			rpc.sendToDevice("m.room.encrypted" as any, { "@u:ex.com": { DEV: {} } })
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				command: "send_to_device",
				data: expect.objectContaining({ encrypted: false }),
			}))
		})

		it("getSpecificRoomState sends correct command", () => {
			rpc.getSpecificRoomState([{ room_id: "!r:ex.com", type: "m.room.member" as any, state_key: "@u:ex.com" }])
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "get_specific_room_state" }))
		})

		it("getRoomState sends correct command with defaults", () => {
			rpc.getRoomState("!r:ex.com")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				command: "get_room_state",
				data: expect.objectContaining({ include_members: false, fetch_members: false, refetch: false }),
			}))
		})

		it("getEvent sends correct command", () => {
			rpc.getEvent("!r:ex.com", "$evt", true)
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				command: "get_event",
				data: expect.objectContaining({ unredact: true }),
			}))
		})

		it("getEventByRowID sends correct command", () => {
			rpc.getEventByRowID(42)
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "get_event_by_rowid" }))
		})

		it("getRelatedEvents sends correct command", () => {
			rpc.getRelatedEvents("!r:ex.com", "$evt", "m.reference" as any, "m.reaction" as any)
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "get_related_events" }))
		})

		it("getStickyEvents sends correct command", () => {
			rpc.getStickyEvents("!r:ex.com")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "get_sticky_events" }))
		})

		it("getMentions sends correct command with defaults", () => {
			rpc.getMentions(12345)
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				command: "get_mentions",
				data: expect.objectContaining({ limit: 50, room_id: undefined }),
			}))
		})

		it("getEventContext sends correct command with default limit", () => {
			rpc.getEventContext("!r:ex.com", "$evt")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				command: "get_event_context",
				data: expect.objectContaining({ limit: 20 }),
			}))
		})

		it("paginateManual sends correct command with defaults", () => {
			rpc.paginateManual("!r:ex.com", "since-token", "b" as any)
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				command: "paginate_manual",
				data: expect.objectContaining({ limit: 50 }),
			}))
		})

		it("paginateManual sends threadRoot", () => {
			rpc.paginateManual("!r:ex.com", "since", "b" as any, { limit: 10, threadRoot: "$thread" })
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				data: expect.objectContaining({ limit: 10, thread_root: "$thread" }),
			}))
		})

		it("searchLocal sends correct command", () => {
			rpc.searchLocal({ query: "test" } as any)
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "search_local" }))
		})

		it("searchServer sends correct command", () => {
			rpc.searchServer({ query: "test" } as any)
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "search_server" }))
		})

		it("paginate sends correct command with defaults", () => {
			rpc.paginate("!r:ex.com", 0)
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				command: "paginate",
				data: expect.objectContaining({ limit: 50, reset: false }),
			}))
		})

		it("getRoomSummary sends correct command", () => {
			rpc.getRoomSummary("!r:ex.com", ["server1"])
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "get_room_summary" }))
		})

		it("getSpaceHierarchy sends correct command with defaults", () => {
			rpc.getSpaceHierarchy("!r:ex.com")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "get_space_hierarchy" }))
		})

		it("joinRoom sends correct command", () => {
			rpc.joinRoom("!r:ex.com", ["server1"], "reason", true)
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				command: "join_room",
				data: expect.objectContaining({ from_invite: true }),
			}))
		})

		it("knockRoom sends correct command", () => {
			rpc.knockRoom("!r:ex.com")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "knock_room" }))
		})

		it("leaveRoom sends correct command", () => {
			rpc.leaveRoom("!r:ex.com", "bye")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				command: "leave_room",
				data: expect.objectContaining({ reason: "bye" }),
			}))
		})

		it("createRoom sends correct command", () => {
			rpc.createRoom({ name: "test" } as any)
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "create_room" }))
		})

		it("getCapabilities sends correct command", () => {
			rpc.getCapabilities()
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "get_capabilities" }))
		})

		it("muteRoom sends correct command", () => {
			rpc.muteRoom("!r:ex.com", true)
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				command: "mute_room",
				data: expect.objectContaining({ muted: true }),
			}))
		})

		it("updatePushRule enable sends correct command", () => {
			rpc.updatePushRule("override" as any, "rule1", "enable")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				command: "update_push_rule",
				data: expect.objectContaining({ action: "enable" }),
			}))
		})

		it("updatePushRule put_actions sends actions", () => {
			rpc.updatePushRule("override" as any, "rule1", "put_actions", { actions: ["notify"] } as any)
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				data: expect.objectContaining({ actions: ["notify"] }),
			}))
		})

		it("updatePushRule put sends new_content", () => {
			rpc.updatePushRule("override" as any, "rule1", "put", { actions: ["dont_notify"] } as any)
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				data: expect.objectContaining({ new_content: expect.objectContaining({ actions: ["dont_notify"] }) }),
			}))
		})

		it("updatePushRule delete sends correct command", () => {
			rpc.updatePushRule("override" as any, "rule1", "delete")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				data: expect.objectContaining({ action: "delete" }),
			}))
		})

		it("resolveAlias sends correct command", () => {
			rpc.resolveAlias("#room:ex.com")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "resolve_alias" }))
		})

		it("discoverHomeserver sends correct command", () => {
			rpc.discoverHomeserver("@u:ex.com")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "discover_homeserver" }))
		})

		it("getLoginFlows sends correct command", () => {
			rpc.getLoginFlows("https://ex.com")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "get_login_flows" }))
		})

		it("oauthRegisterClient sends correct command", () => {
			rpc.oauthRegisterClient("https://ex.com", { client_name: "test" } as any)
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "oauth_register_client" }))
		})

		it("oauthGetAuthorizationURL sends correct command", () => {
			rpc.oauthGetAuthorizationURL({} as any)
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "oauth_get_authorization_url" }))
		})

		it("oauthExchangeToken sends correct command", () => {
			rpc.oauthExchangeToken({} as any)
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "oauth_exchange_token" }))
		})

		it("oauthGenerateDeviceCode sends correct command", () => {
			rpc.oauthGenerateDeviceCode({} as any)
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "oauth_generate_device_code" }))
		})

		it("oauthPollDeviceCode sends correct command", () => {
			rpc.oauthPollDeviceCode("https://ex.com", "code123", "client1")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "oauth_poll_device_code" }))
		})

		it("login sends correct command", () => {
			rpc.login("https://ex.com", "user", "pass")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "login" }))
		})

		it("loginCustom sends correct command", () => {
			rpc.loginCustom("https://ex.com", { type: "m.login.token" } as any)
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "login_custom" }))
		})

		it("verify sends correct command", () => {
			rpc.verify("recovery-key")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "verify" }))
		})

		it("generateRecoveryKey sends correct command", () => {
			rpc.generateRecoveryKey("passphrase")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "generate_recovery_key" }))
		})

		it("resetEncryption sends correct command", () => {
			rpc.resetEncryption({ recovery_key: "key" } as any, "password")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({
				command: "reset_encryption",
				data: expect.objectContaining({ account_password: "password" }),
			}))
		})

		it("requestOpenIDToken sends correct command", () => {
			rpc.requestOpenIDToken()
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "request_openid_token" }))
		})

		it("registerPush sends correct command", () => {
			rpc.registerPush({ type: "web", device_id: "dev1", data: {}, expiration: 100 } as any)
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "register_push" }))
		})

		it("getTurnServers sends correct command", () => {
			rpc.getTurnServers()
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "get_turn_servers" }))
		})

		it("getRTCTransports sends correct command", () => {
			rpc.getRTCTransports()
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "get_rtc_transports" }))
		})

		it("getMediaConfig sends correct command", () => {
			rpc.getMediaConfig()
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "get_media_config" }))
		})

		it("setListenToDevice sends correct command", () => {
			rpc.setListenToDevice(true)
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "listen_to_device" }))
		})

		it("calculateRoomID sends correct command", () => {
			rpc.calculateRoomID(12345, { content: "test" })
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "calculate_room_id" }))
		})

		it("rerequestSession sends correct command", () => {
			rpc.rerequestSession("!r:ex.com", "session1", "@u:ex.com")
			expect(rpc.send).toHaveBeenCalledWith(expect.objectContaining({ command: "rerequest_session" }))
		})
	})

	describe("rpcMediaUpload", () => {
		it("defaults to false", () => {
			expect(rpc.rpcMediaUpload).toBe(false)
		})
	})

	describe("connect/event dispatchers", () => {
		it("connect dispatcher is a CachedEventDispatcher", () => {
			expect(rpc.connect).toBeDefined()
			expect(typeof rpc.connect.emit).toBe("function")
			expect(typeof rpc.connect.listen).toBe("function")
		})

		it("event dispatcher is an EventDispatcher", () => {
			expect(rpc.event).toBeDefined()
			expect(typeof rpc.event.emit).toBe("function")
			expect(typeof rpc.event.listen).toBe("function")
		})
	})
})
