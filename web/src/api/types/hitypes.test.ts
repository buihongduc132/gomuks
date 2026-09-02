// gomuks - A Matrix client written in Go.
// Copyright (C) 2024 Tulir Asokan
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
import {
	RoomNameQuality,
	UnreadType,
	roomStateGUIDToString,
	stringToRoomStateGUID,
} from "./hitypes.ts"

describe("RoomNameQuality", () => {
	it("has correct ascending quality values", () => {
		expect(RoomNameQuality.Nil).toBe(0)
		expect(RoomNameQuality.Participants).toBe(1)
		expect(RoomNameQuality.CanonicalAlias).toBe(2)
		expect(RoomNameQuality.Explicit).toBe(3)
	})
})

describe("UnreadType", () => {
	it("has bitmask values", () => {
		expect(UnreadType.None).toBe(0b0000)
		expect(UnreadType.Normal).toBe(0b0001)
		expect(UnreadType.Notify).toBe(0b0010)
		expect(UnreadType.Highlight).toBe(0b0100)
		expect(UnreadType.Sound).toBe(0b1000)
	})
})

describe("roomStateGUIDToString", () => {
	it("produces type/state_key/room_id string with encoded components", () => {
		const result = roomStateGUIDToString({
			room_id: "!room:example.com",
			type: "m.room.member",
			state_key: "@user:example.com",
		})
		// encodeURIComponent encodes both : and @
		expect(result).toBe("!room%3Aexample.com/m.room.member/%40user%3Aexample.com")
	})

	it("encodes special characters in state_key", () => {
		const result = roomStateGUIDToString({
			room_id: "!foo",
			type: "custom.type",
			state_key: "a b/c",
		})
		expect(result).toBe("!foo/custom.type/a%20b%2Fc")
	})
})

describe("stringToRoomStateGUID", () => {
	it("round-trips roomStateGUIDToString output", () => {
		const guid = {
			room_id: "!room:example.com",
			type: "m.room.member",
			state_key: "@user:example.com",
		}
		expect(stringToRoomStateGUID(roomStateGUIDToString(guid))).toEqual(guid)
	})

	it("returns undefined for empty/absent input", () => {
		expect(stringToRoomStateGUID(undefined)).toBeUndefined()
		expect(stringToRoomStateGUID(null)).toBeUndefined()
		expect(stringToRoomStateGUID("")).toBeUndefined()
	})

	it("returns undefined when a component is missing", () => {
		expect(stringToRoomStateGUID("onlyroom")).toBeUndefined()
		expect(stringToRoomStateGUID("room/type")).toBeUndefined()
	})

	it("returns undefined when state_key is empty", () => {
		// type is present but state_key splits to "" which is undefined-checkable
		const result = stringToRoomStateGUID("!room/type/")
		expect(result).toBeDefined()
		expect(result?.state_key).toBe("")
	})

	it("parses room and type without decoding issues", () => {
		const result = stringToRoomStateGUID("!room:example.com/m.room.topic/plain")
		expect(result).toEqual({
			room_id: "!room:example.com",
			type: "m.room.topic",
			state_key: "plain",
		})
	})
})
