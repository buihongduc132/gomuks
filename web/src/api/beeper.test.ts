import { describe, it, expect, vi, beforeEach } from "vitest"
import { doStartLogin, doRequestCode, doSubmitCode } from "./beeper"

const headers = {
	"Authorization": "Bearer BEEPER-PRIVATE-API-PLEASE-DONT-USE",
	"Content-Type": "application/json",
}

beforeEach(() => {
	vi.restoreAllMocks()
})

describe("doStartLogin", () => {
	it("returns request id on success", async () => {
		vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
			ok: true,
			status: 200,
			json: () => Promise.resolve({ request: "req-123" }),
		}))
		const result = await doStartLogin("example.com")
		expect(result).toBe("req-123")
		expect(fetch).toHaveBeenCalledWith("https://api.example.com/user/login", {
			method: "POST",
			body: "{}",
			headers,
		})
	})

	it("throws when no request returned", async () => {
		vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
			ok: true,
			status: 200,
			json: () => Promise.resolve({}),
		}))
		await expect(doStartLogin("example.com")).rejects.toThrow("No request ID returned")
	})

	it("throws on HTTP error with json body", async () => {
		vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
			ok: false,
			status: 400,
			json: () => Promise.resolve({ error: "bad request" }),
		}))
		await expect(doStartLogin("example.com")).rejects.toThrow("HTTP 400 / bad request")
	})

	it("throws on HTTP error without json body", async () => {
		vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
			ok: false,
			status: 500,
			json: () => Promise.reject(new Error("no json")),
		}))
		await expect(doStartLogin("example.com")).rejects.toThrow("HTTP 500")
	})
})

describe("doRequestCode", () => {
	it("succeeds on ok response", async () => {
		vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
			ok: true,
			status: 200,
			json: () => Promise.resolve({}),
		}))
		await expect(doRequestCode("example.com", "req-1", "a@b.com")).resolves.toBeUndefined()
		expect(fetch).toHaveBeenCalledWith("https://api.example.com/user/login/email", {
			method: "POST",
			body: JSON.stringify({ email: "a@b.com", request: "req-1", appType: "gomuks", onlyExistingAccounts: true }),
			headers,
		})
	})

	it("throws on HTTP error with json", async () => {
		vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
			ok: false,
			status: 403,
			json: () => Promise.resolve({ error: "forbidden" }),
		}))
		await expect(doRequestCode("example.com", "req-1", "a@b.com")).rejects.toThrow("HTTP 403 / forbidden")
	})

	it("throws on HTTP error without json", async () => {
		vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
			ok: false,
			status: 502,
			json: () => Promise.reject(new Error("no json")),
		}))
		await expect(doRequestCode("example.com", "req-1", "a@b.com")).rejects.toThrow("HTTP 502")
	})
})

describe("doSubmitCode", () => {
	it("returns token on success", async () => {
		vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
			ok: true,
			status: 200,
			json: () => Promise.resolve({ token: "tok-abc" }),
		}))
		const result = await doSubmitCode("example.com", "req-1", "code123")
		expect(result).toBe("tok-abc")
		expect(fetch).toHaveBeenCalledWith("https://api.example.com/user/login/response", {
			method: "POST",
			body: JSON.stringify({ response: "code123", request: "req-1", appType: "gomuks", onlyExistingAccounts: true }),
			headers,
		})
	})

	it("throws on HTTP error with json", async () => {
		vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
			ok: false,
			status: 401,
			json: () => Promise.resolve({ error: "invalid code" }),
		}))
		await expect(doSubmitCode("example.com", "req-1", "bad")).rejects.toThrow("HTTP 401 / invalid code")
	})

	it("throws on HTTP error without json", async () => {
		vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
			ok: false,
			status: 500,
			json: () => Promise.reject(new Error("no json")),
		}))
		await expect(doSubmitCode("example.com", "req-1", "bad")).rejects.toThrow("HTTP 500")
	})

	it("throws when no token returned", async () => {
		vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
			ok: true,
			status: 200,
			json: () => Promise.resolve({}),
		}))
		await expect(doSubmitCode("example.com", "req-1", "code")).rejects.toThrow("No token returned")
	})
})
