package main

import (
	"context"
	"fmt"

	"github.com/vertex-lab/relay/pkg/eventstore"

	"github.com/fiatjaf/khatru"
	"github.com/nbd-wtf/go-nostr/nip86"
)

// RelayManagementInit() initializes the NIP-86 relay management API.
func RelayManagementInit(
	ctx context.Context,
	db *eventstore.Store,
	relay *khatru.Relay) error {

	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS authorized_keys(
		pubkey TEXT NOT NULL,
		reason TEXT
		);`)
	if err != nil {
		return err
	}

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
		rows, err := db.QueryContext(ctx, "SELECT * FROM authorized_keys")
		if err != nil {
			return nil, fmt.Errorf("failed to lookup authorized keys: %w", err)
		}

		var reasons []nip86.PubKeyReason
		for rows.Next() {
			var pubkey, reason string
			if err := rows.Scan(&pubkey, &reason); err != nil {
				return nil, fmt.Errorf("failed to lookup authorized keys: %w", err)
			}

			reasons = append(reasons, nip86.PubKeyReason{PubKey: pubkey, Reason: reason})
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

	return nil
}
