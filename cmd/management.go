package main

import (
	"context"

	"github.com/fiatjaf/khatru"
	"github.com/nbd-wtf/go-nostr/nip86"
)

// NIP-86

func RelayManagementInit(relay *khatru.Relay) {

	Db.Exec(`CREATE TABLE IF NOT EXISTS authorized_keys(
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

		err := Db.Select(&reasons, "SELECT pubkey, reason FROM authorized_keys")
		if err != nil {
			return nil, err
		}
		return reasons, nil
	}

	relay.ManagementAPI.AllowPubKey = func(ctx context.Context, pubkey string, reason string) error {
		_, err := Db.Exec(`INSERT INTO authorized_keys (pubkey, reason) VALUES (?, ?)`, pubkey, reason)
		if err == nil {
			return nil
		}
		return err
	}

	relay.ManagementAPI.BanPubKey = func(ctx context.Context, pubkey string, reason string) error {
		_, err := Db.Exec(`DELETE FROM authorized_keys WHERE pubkey = ?`, pubkey)
		if err == nil {
			return nil
		}
		return err
	}
}
