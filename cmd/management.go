package main

import (
	"context"

	"github.com/fiatjaf/eventstore/sqlite3"
	"github.com/fiatjaf/khatru"
	"github.com/nbd-wtf/go-nostr/nip86"
)

// RelayManagementInit() initializes the NIP-86 management for the relay.
func RelayManagementInit(db *sqlite3.SQLite3Backend, relay *khatru.Relay) {

	db.Exec(`CREATE TABLE IF NOT EXISTS authorized_keys(
		pubkey TEXT NOT NULL,
		reason TEXT
		);`)

	relay.ManagementAPI.RejectAPICall = append(relay.ManagementAPI.RejectAPICall,
		func(ctx context.Context, mp nip86.MethodParams) (reject bool, msg string) {
			user := khatru.GetAuthed(ctx)
			if user != relay.Info.PubKey {
				return true, "Fuck off"
			}
			return false, ""
		},
	)

	relay.ManagementAPI.ListAllowedPubKeys = func(ctx context.Context) ([]nip86.PubKeyReason, error) {
		var reasons []nip86.PubKeyReason

		if err := db.Select(&reasons, "SELECT pubkey, reason FROM authorized_keys"); err != nil {
			return nil, err
		}

		return reasons, nil
	}

	relay.ManagementAPI.AllowPubKey = func(ctx context.Context, pubkey string, reason string) error {
		_, err := db.Exec(`INSERT INTO authorized_keys (pubkey, reason) VALUES (?, ?)`, pubkey, reason)
		return err
	}

	relay.ManagementAPI.BanPubKey = func(ctx context.Context, pubkey string, reason string) error {
		_, err := db.Exec(`DELETE FROM authorized_keys WHERE pubkey = ?`, pubkey)
		return err
	}
}
