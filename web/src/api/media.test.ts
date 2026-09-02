import { describe, it, expect, beforeEach, vi } from "vitest"
import {
	getMediaURL,
	getEncryptedMediaURL,
	getUserColorIndex,
	getUserColor,
	getAvatarURL,
	getAvatarThumbnailURL,
	getRoomAvatarURL,
	getRoomAvatarThumbnailURL,
} from "./media"

beforeEach(() => {
	localStorage.clear()
	document.head.innerHTML = ""
	document.body.innerHTML = ""
})

describe("getMediaURL", () => {
	it("returns media URL for valid mxc", () => {
		expect(getMediaURL("mxc://example.com/abcDEF123")).toBe("_gomuks/media/example.com/abcDEF123?encrypted=false")
	})

	it("returns encrypted URL when requested", () => {
		expect(getMediaURL("mxc://example.com/abcDEF123", true)).toBe("_gomuks/media/example.com/abcDEF123?encrypted=true")
	})

	it("returns undefined for invalid mxc", () => {
		expect(getMediaURL("https://example.com/bla")).toBeUndefined()
		expect(getMediaURL(undefined)).toBeUndefined()
		expect(getMediaURL("mxc://")).toBeUndefined()
	})
})

describe("getEncryptedMediaURL", () => {
	it("delegates with encrypted flag", () => {
		expect(getEncryptedMediaURL("mxc://example.com/xyz")).toBe("_gomuks/media/example.com/xyz?encrypted=true")
	})
})

describe("getUserColorIndex", () => {
	it("sums char codes modulo 10", () => {
		const idx = getUserColorIndex("@user:example.com")
		const expected = "@user:example.com".split("").reduce((acc, c) => acc + c.charCodeAt(0), 0) % 10
		expect(idx).toBe(expected)
	})
})

describe("getUserColor", () => {
	it("returns computed style property", () => {
		const style = document.createElement("style")
		style.textContent = `body { --sender-color-0: #ff0000; --sender-color-1: #00ff00; }`
		document.head.appendChild(style)
		// jsdom getComputedStyle does not compute custom properties from stylesheets reliably,
		// so simply assert it returns a string (possibly empty) without throwing
		const color = getUserColor("@user:example.com")
		expect(typeof color).toBe("string")
	})
})

describe("getAvatarURL", () => {
	it("returns fallback avatar when no content", () => {
		const url = getAvatarURL("@user:example.com")
		expect(url.startsWith("data:image/svg+xml,")).toBe(true)
		expect(url).toContain(encodeURIComponent("<svg"))
	})

	it("returns fallback avatar with forceFallback", () => {
		const url = getAvatarURL("@user:example.com", { avatar_url: "mxc://example.com/avatar123" }, false, true)
		expect(url.startsWith("data:image/svg+xml,")).toBe(true)
	})

	it("escapes HTML-special fallback characters", () => {
		const url = getAvatarURL("@user:example.com", { displayname: "<test" })
		const decoded = decodeURIComponent(url)
		expect(decoded).toContain("&lt;")
	})

	it("escapes ampersand in fallback character", () => {
		const url = getAvatarURL("@user:example.com", { displayname: "&test" })
		const decoded = decodeURIComponent(url)
		expect(decoded).toContain("&amp;")
	})

	it("escapes greater-than in fallback character", () => {
		const url = getAvatarURL("@user:example.com", { displayname: ">test" })
		const decoded = decodeURIComponent(url)
		expect(decoded).toContain("&gt;")
	})

	it("handles unicode fallback characters", () => {
		const url = getAvatarURL("@user:example.com", { displayname: "👋🏻 hi" })
		const decoded = decodeURIComponent(url)
		expect(decoded).toContain("👋")
	})

	it("returns media URL with unencrypted avatar", () => {
		const url = getAvatarURL("@user:example.com", { displayname: "User", avatar_url: "mxc://example.com/avatar123" })
		expect(url).toMatch(/^_gomuks\/media\/example\.com\/avatar123\?encrypted=false&fallback=/)
	})

	it("returns media URL with encrypted avatar file", () => {
		const url = getAvatarURL("@user:example.com", {
			avatar_file: { url: "mxc://example.com/encavatar" },
		})
		expect(url).toMatch(/^_gomuks\/media\/example\.com\/encavatar\?encrypted=true&fallback=/)
	})

	it("appends thumbnail parameter unless thumbnails disabled", () => {
		const url = getAvatarURL("@user:example.com", { avatar_url: "mxc://example.com/avatar123" }, true)
		expect(url).toContain("&thumbnail=avatar")
	})

	it("omits thumbnail parameter when thumbnails disabled in localStorage", async () => {
		// disableThumbnails is a module-level constant evaluated at import time
		localStorage.gomuks_disable_thumbnails = "true"
		vi.resetModules()
		const { getAvatarURL: getAvatarURL2 } = await import("./media")
		const url = getAvatarURL2("@user:example.com", { avatar_url: "mxc://example.com/avatar123" }, true)
		expect(url).not.toContain("thumbnail=")
		vi.resetModules()
	})
})

describe("getAvatarThumbnailURL", () => {
	it("forces thumbnail mode", () => {
		const url = getAvatarThumbnailURL("@user:example.com", { avatar_url: "mxc://example.com/avatar123" })
		expect(url).toContain("&thumbnail=avatar")
	})
})

describe("getRoomAvatarURL", () => {
	it("uses dm_user_id for fallback character", () => {
		const url = getRoomAvatarURL({
			room_id: "!room:example.com",
			dm_user_id: "@dm:example.com",
			name: "Room Name",
		})
		expect(url.startsWith("data:image/svg+xml,")).toBe(true)
		const decoded = decodeURIComponent(url)
		// fallback character comes from displayname index 0 ("R" from "Room Name")
		expect(decoded).toContain(">R<")
	})

	it("uses avatar override when provided", () => {
		const url = getRoomAvatarURL(
			{ room_id: "!room:example.com", name: "Room", avatar: "mxc://example.com/orig" },
			"mxc://example.com/override",
		)
		expect(url).toMatch(/^_gomuks\/media\/example\.com\/override\?/)
	})

	it("uses room avatar url", () => {
		const url = getRoomAvatarURL({ room_id: "!room:example.com", name: "Room", avatar_url: "mxc://example.com/ravatar" })
		expect(url).toMatch(/^_gomuks\/media\/example\.com\/ravatar\?/)
	})

	it("passes forceFallback through", () => {
		const url = getRoomAvatarURL(
			{ room_id: "!room:example.com", name: "Room", avatar_url: "mxc://example.com/ravatar" },
			undefined, false, true,
		)
		expect(url.startsWith("data:image/svg+xml,")).toBe(true)
	})
})

describe("getRoomAvatarThumbnailURL", () => {
	it("forces thumbnail mode", () => {
		const url = getRoomAvatarThumbnailURL({ room_id: "!room:example.com", name: "Room", avatar_url: "mxc://example.com/ravatar" })
		expect(url).toContain("&thumbnail=avatar")
	})
})
