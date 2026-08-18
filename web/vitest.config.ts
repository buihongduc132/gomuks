// NOTE: this file intentionally does not import `defineConfig` from "vitest/config".
// vite 8.2.1's rolldown config bundler fails to externalize transitive `node:url`
// imports from vitest/config ("Cannot find package 'node'" startup error), so the
// config is a plain object with zero bundlable imports.
export default {
	test: {
		environment: "jsdom",
		include: ["src/**/*.test.{ts,tsx}"],
		coverage: {
			provider: "v8",
			reporter: ["text", "json-summary", "lcov"],
			// Only ever match .ts/.tsx sources. The v8 provider feeds every "uncovered"
			// file through rolldown's parser, so a stray .go/.json/.js asset in these
			// dirs (e.g. src/api/wasm/go_wasm_exec.js) aborts the whole coverage report
			// with a RolldownError.
			include: [
				"src/util/**/*.{ts,tsx}",
				"src/ui/keybindings.ts",
				"src/api/**/*.{ts,tsx}",
				"src/ui/util/**/*.{ts,tsx}",
			],
			exclude: ["src/**/*.test.{ts,tsx}", "src/api/types/commandtestdata/**", "coverage/**"],
			thresholds: { lines: 0, statements: 0, functions: 0, branches: 0 },
		},
	},
	resolve: {
		alias: {
			"@": "/src",
		},
	},
}
