// gomuks - A Matrix client written in Go.
// Copyright (C) 2024 Tulir Asokan
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
import { describe, expect, test } from "vitest"
import type { DBInvitedRoom, StrippedStateEvent } from "../types"
import toSearchableString from "@/util/searchablestring.ts"
import { InvitedRoomStore } from "./invitedroom.ts"

const USER_ID = "@me:example.com"
const INVITER = "@inviter:example.com"

function makeParent(userID: string = USER_ID) {
	return { userID } as never
}

function makeInvite(
	inviteState: StrippedStateEvent[],
	overrides: Partial<DBInvitedRoom> = {},
): DBInvitedRoom {
	return {
		room_id: "!invite:example.com",
		created_at: 1700000000000,
		invite_state: inviteState,
		...overrides,
	}
}

function makeState(
	type: string, stateKey: string, sender: string, content: Record<string, unknown>,
): StrippedStateEvent {
	return { type, state_key: stateKey, sender, content }
}

describe("InvitedRoomStore", () => {
	test("basic invite with room name", () => {
		const meta = makeInvite([
			makeState("m.room.name", "", INVITER, { name: "Named Room" }),
			makeState("m.room.member", USER_ID, INVITER, { membership: "invite" }),
		])
		const room = new InvitedRoomStore(meta, makeParent())
		expect(room.name).toBe("Named Room")
		expect(room.is_invite).toBe(true)
		expect(room.room_id).toBe("!invite:example.com")
		expect(room.membership).toBe("invite")
	})

	test("unnamed room falls back to Unnamed room", () => {
		const meta = makeInvite([])
		const room = new InvitedRoomStore(meta, makeParent())
		expect(room.name).toBe("Unnamed room")
	})

	test("parses canonical_alias", () => {
		const meta = makeInvite([
			makeState("m.room.canonical_alias", "", INVITER, { alias: "#room:example.com" }),
		])
		const room = new InvitedRoomStore(meta, makeParent())
		expect(room.canonical_alias).toBe("#room:example.com")
	})

	test("parses topic", () => {
		const meta = makeInvite([
			makeState("m.room.topic", "", INVITER, { topic: "A topic" }),
		])
		const room = new InvitedRoomStore(meta, makeParent())
		expect(room.topic).toBe("A topic")
	})

	test("parses avatar", () => {
		const meta = makeInvite([
			makeState("m.room.avatar", "", INVITER, { url: "mxc://example.com/abc" }),
		])
		const room = new InvitedRoomStore(meta, makeParent())
		expect(room.avatar).toBe("mxc://example.com/abc")
		expect(room.avatar_url).toBe("mxc://example.com/abc")
	})

	test("parses encryption", () => {
		const meta = makeInvite([
			makeState("m.room.encryption", "", INVITER, { algorithm: "m.megolm.v1.aes-sha2" }),
		])
		const room = new InvitedRoomStore(meta, makeParent())
		expect(room.encryption).toBe("m.megolm.v1.aes-sha2")
	})

	test("parses room version", () => {
		const meta = makeInvite([
			makeState("m.room.create", "", INVITER, { version: "11" }),
		])
		const room = new InvitedRoomStore(meta, makeParent())
		expect(room.room_version).toBe("11")
	})

	test("parses join rules", () => {
		const meta = makeInvite([
			makeState("m.room.join_rules", "", INVITER, { join_rule: "knock" }),
		])
		const room = new InvitedRoomStore(meta, makeParent())
		expect(room.join_rule).toBe("knock")
	})

	test("identifies inviter from own member event", () => {
		const meta = makeInvite([
			makeState("m.room.member", USER_ID, INVITER, { membership: "invite" }),
			makeState("m.room.member", INVITER, INVITER, {
				membership: "join", displayname: "The Inviter",
			}),
		])
		const room = new InvitedRoomStore(meta, makeParent())
		expect(room.invited_by).toBe(INVITER)
		expect(room.inviter_profile?.displayname).toBe("The Inviter")
	})

	test("DM invite with no name/avatar/topic uses inviter as name", () => {
		const meta = makeInvite([
			makeState("m.room.member", USER_ID, INVITER, { membership: "invite", is_direct: true }),
			makeState("m.room.member", INVITER, INVITER, {
				membership: "join", displayname: "Direct Person",
			}),
			makeState("m.room.join_rules", "", INVITER, { join_rule: "invite" }),
		])
		const room = new InvitedRoomStore(meta, makeParent())
		expect(room.is_direct).toBe(true)
		expect(room.dm_user_id).toBe(INVITER)
		expect(room.name).toBe("Direct Person")
	})

	test("non-direct invite with no name stays unnamed", () => {
		const meta = makeInvite([
			makeState("m.room.member", USER_ID, INVITER, { membership: "invite" }),
			makeState("m.room.join_rules", "", INVITER, { join_rule: "invite" }),
		])
		const room = new InvitedRoomStore(meta, makeParent())
		expect(room.is_direct).toBe(false)
		expect(room.dm_user_id).toBeUndefined()
		expect(room.name).toBe("Unnamed room")
	})

	test("sorting_timestamp is based on created_at", () => {
		const meta = makeInvite([], { created_at: 5000 })
		const room = new InvitedRoomStore(meta, makeParent())
		expect(room.sorting_timestamp).toBe(1000000000000000 + 5000)
	})

	test("getters return expected values", () => {
		const meta = makeInvite([])
		const room = new InvitedRoomStore(meta, makeParent())
		expect(room.num_joined_members).toBe(0)
		expect(room.room_type).toBe("")
		expect(room.world_readable).toBe(false)
		expect(room.guest_can_join).toBe(false)
		expect(room.unread_messages).toBe(0)
		expect(room.unread_notifications).toBe(0)
		expect(room.unread_highlights).toBe(1)
		expect(room.marked_unread).toBe(true)
		expect(room.low_priority).toBe(false)
	})

	test("search_name is searchable version of name", () => {
		const meta = makeInvite([
			makeState("m.room.name", "", INVITER, { name: "Searchable Room!" }),
		])
		const room = new InvitedRoomStore(meta, makeParent())
		expect(room.search_name).toBe(toSearchableString("Searchable Room!"))
	})
})
