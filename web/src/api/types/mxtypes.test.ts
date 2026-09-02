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
import { ContentWarningType } from "./mxtypes.ts"

describe("ContentWarningType", () => {
	it("has correct enum values", () => {
		expect(ContentWarningType.Spoiler).toBe("town.robin.msc3725.spoiler")
		expect(ContentWarningType.NSFW).toBe("town.robin.msc3725.nsfw")
		expect(ContentWarningType.Graphic).toBe("town.robin.msc3725.graphic")
		expect(ContentWarningType.Medical).toBe("town.robin.msc3725.medical")
	})

	it("has exactly 4 members", () => {
		const values = Object.values(ContentWarningType)
		expect(values).toHaveLength(4)
	})
})
