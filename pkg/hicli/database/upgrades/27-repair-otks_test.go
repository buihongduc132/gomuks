//go:build go1.27 || goexperiment.jsonv2

package upgrades

import (
	"context"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/util/dbutil"
)

func TestUpgradeV27HandlesMissingCryptoAccount(t *testing.T) {
	cases := []struct {
		name      string
		table     bool
		cryptoRow bool
		shared    bool
		loggedIn  bool
		wantReset bool
	}{
		{name: "fresh_no_crypto_table"},
		{name: "fresh_empty_crypto_table", table: true},
		{name: "unshared_crypto_account", table: true, cryptoRow: true},
		{name: "shared_no_account", table: true, cryptoRow: true, shared: true},
		{name: "shared_logged_in", table: true, cryptoRow: true, shared: true, loggedIn: true, wantReset: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			db, err := dbutil.NewWithDialect(filepath.Join(t.TempDir(), "migration.db"), "sqlite3")
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if _, err := db.Exec(ctx, "CREATE TABLE account (id INTEGER)"); err != nil {
				t.Fatal(err)
			}
			if tc.loggedIn {
				if _, err := db.Exec(ctx, "INSERT INTO account (id) VALUES (1)"); err != nil {
					t.Fatal(err)
				}
			}
			if tc.table {
				if _, err := db.Exec(ctx, "CREATE TABLE crypto_account (shared BOOLEAN)"); err != nil {
					t.Fatal(err)
				}
				if tc.cryptoRow {
					if _, err := db.Exec(ctx, "INSERT INTO crypto_account (shared) VALUES (?)", tc.shared); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := runUpgradeV27(ctx, db); err != nil {
				t.Fatal(err)
			}
			got, err := db.TableExists(ctx, "otks_need_reset")
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.wantReset {
				t.Fatalf("otks_need_reset exists: got %v, want %v", got, tc.wantReset)
			}
		})
	}
}
