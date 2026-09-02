// gomuks - A Matrix client written in Go.
// Copyright (C) 2026 Tulir Asokan
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
import { describe, expect, it } from "vitest"
import FakeCommands, { isFakeCommand } from "./fakecommands.ts"

describe("isFakeCommand", () => {
	it.each([
		["/plain hello", true],
		["/me waves", true],
		["/notice hello", true],
		["/rainbow text", true],
		["/html <b>bold</b>", true],
		["/htmlmd mixed", true],
		["/timestamp 1234567890", true],
		["/unencrypted msg", true],
		["/rawinputbody text", true],
		["/unknown cmd", false],
		["/plainx", false],
		["plain hello", false],
		["", false],
		["/plain", false], // no trailing space
		["/me", false],
	])("isFakeCommand(%j) === %s", (input, expected) => {
		expect(isFakeCommand(input)).toBe(expected)
	})
})

describe("FakeCommands default export", () => {
	it("contains expected command names", () => {
		const names = FakeCommands.map(c => c.command)
		expect(names).toContain("plain")
		expect(names).toContain("html")
		expect(names).toContain("rainbow")
		expect(names).toContain("htmlmd")
		expect(names).toContain("me")
		expect(names).toContain("notice")
		expect(names).toContain("unencrypted")
		expect(names).toContain("rawinputbody")
		expect(names).toContain("timestamp")
	})

	it("all commands are marked as fake and from @gomuks", () => {
		for (const cmd of FakeCommands) {
			expect(cmd.source).toBe("@gomuks")
			expect(cmd.fake).toBe(true)
		}
	})

	it("each command has a text parameter", () => {
		for (const cmd of FakeCommands) {
			const textParam = cmd.parameters.find(p => p.key === "text")
			expect(textParam).toBeDefined()
			expect(textParam?.schema).toEqual({
				schema_type: "primitive",
				type: "string",
			})
		}
	})

	it("timestamp command has an extra integer parameter", () => {
		const timestamp = FakeCommands.find(c => c.command === "timestamp")!
		const tsParam = timestamp.parameters.find(p => p.key === "timestamp")
		expect(tsParam).toBeDefined()
		expect(tsParam?.schema).toEqual({
			schema_type: "primitive",
			type: "integer",
		})
	})

	it("each command has a description with m.text", () => {
		for (const cmd of FakeCommands) {
			expect(cmd.description?.["m.text"]).toBeDefined()
			expect(cmd.description!["m.text"].length).toBeGreaterThan(0)
		}
	})
})
