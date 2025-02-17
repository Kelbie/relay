package eventstore

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/nbd-wtf/go-nostr"
)

var event1 = nostr.Event{
	ID:        "f7a73d54e45714f5e3ca97b789dfc7898e7dd31f77981989d71a54030e627ff6",
	Kind:      0,
	PubKey:    "f683e87035f7ad4f44e0b98cfbd9537e16455a92cd38cefc4cb31db7557f5ef2",
	CreatedAt: 1739547448,
	Sig:       "51a89ee1e24d83bd8e9209daf6a38245c974b49206ecb66fe156c9d7875c782f653b40cd73582f6bc9de5d1db497b925a13a828d521f8b78982fea359206e4e8",
	Content:   "{\"name\":\"pippellia\",\"nip05\":\"pip@vertexlab.io\",\"about\":\"simplifying social graph analysis so you can focus on building great experiences https://vertexlab.io/\",\"lud16\":\"whitebat1@primal.net\",\"display_name\":\"Pip the social graph guy\",\"picture\":\"https://m.primal.net/IfSZ.jpg\",\"banner\":\"https://m.primal.net/IfSc.png\",\"website\":\"pippellia.com\",\"displayName\":\"Pip the social graph guy\",\"pubkey\":\"f683e87035f7ad4f44e0b98cfbd9537e16455a92cd38cefc4cb31db7557f5ef2\",\"npub\":\"npub176p7sup477k5738qhxx0hk2n0cty2k5je5uvalzvkvwmw4tltmeqw7vgup\",\"created_at\":1738783677}",
}

var profile1 = Profile{
	ID:          event1.ID,
	Pubkey:      event1.PubKey,
	Name:        "pippellia",
	DisplayName: "Pip the social graph guy",
	About:       "simplifying social graph analysis so you can focus on building great experiences https://vertexlab.io/",
	Website:     "pippellia.com",
	Nip05:       "pip@vertexlab.io",
}

func TestEvent1(t *testing.T) {
	if !event1.CheckID() {
		t.Fatalf("ID is bad")
	}

	match, err := event1.CheckSignature()
	if err != nil || !match {
		t.Fatalf("signature is invalid: %v", err)
	}
}

func TestSave(t *testing.T) {
	ctx := context.Background()
	const URL = "test.sqlite"

	store, err := New(URL)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(URL)

	// step 1; save into the event
	if err := store.Save(ctx, &event1); err != nil {
		t.Fatal(err)
	}

	var saved nostr.Event
	row := store.DB.QueryRowContext(ctx, "SELECT * FROM events WHERE id = ?", event1.ID)
	err = row.Scan(&saved.ID, &saved.PubKey, &saved.CreatedAt, &saved.Kind, &saved.Tags, &saved.Content, &saved.Sig)

	if err != nil {
		t.Fatalf("failed to query for eventID %s: %v", event1.ID, err)
	}

	// step 2. check the event is the same after saving and querying
	if !reflect.DeepEqual(saved, event1) {
		t.Errorf("the event is not what it was before!")
		t.Fatalf(" expected %v\n got %v", event1, saved)
	}

	var profile Profile
	row = store.DB.QueryRowContext(ctx, "SELECT * FROM profiles_fts WHERE id = ?", event1.ID)
	err = row.Scan(&profile.ID, &profile.Pubkey, &profile.Name, &profile.DisplayName, &profile.About, &profile.Website, &profile.Nip05)

	if err != nil {
		t.Fatalf("failed to query for eventID %s in profiles_fts: %v", event1.ID, err)
	}

	// step 3. check the profile has been saved correctly
	if !reflect.DeepEqual(profile, profile1) {
		t.Fatalf("expected profile %v, got %v", profile1, profile)
	}
}

func TestDelete(t *testing.T) {
	ctx := context.Background()
	const URL = "test.sqlite"

	store, err := New(URL)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(URL)

	// step 1; save into the events table
	if err := store.Save(ctx, &event1); err != nil {
		t.Fatal(err)
	}

	// step 2; test delete
	if err := store.Delete(ctx, event1.ID); err != nil {
		t.Fatal(err)
	}

	// step 3; check that the events and profiles_fts tables are empty
	var rowsCount int
	err = store.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM events").Scan(&rowsCount)
	if err != nil {
		t.Fatalf("failed to count the rows in the events table")
	}

	if rowsCount != 0 {
		t.Fatalf("expected empty events table, but found %d row(s)", rowsCount)
	}

	err = store.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM profiles_fts").Scan(&rowsCount)
	if err != nil {
		t.Fatalf("failed to count the rows in the profiles_fts table")
	}

	if rowsCount != 0 {
		t.Fatalf("expected empty profiles_fts table, but found %d row(s)", rowsCount)
	}
}

var event10 = nostr.Event{ID: "bbb", Kind: 0, PubKey: "key", CreatedAt: 10, Sig: "xx", Content: "{}"}
var event100 = nostr.Event{ID: "aaa", Kind: 0, PubKey: "key", CreatedAt: 100, Sig: "xx", Content: "{}"}

func TestReplace(t *testing.T) {
	testCases := []struct {
		name        string
		storedEvent nostr.Event
		newEvent    nostr.Event
		replaced    bool
	}{
		{
			name:        "no replace (event is not newer)",
			storedEvent: event100,
			newEvent:    event10,
			replaced:    false,
		},
		{
			name:        "valid replace (event is newer)",
			storedEvent: event10,
			newEvent:    event100,
			replaced:    true,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			const URL = "test.sqlite"

			store, err := New(URL)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(URL)

			if err := store.Save(ctx, &test.storedEvent); err != nil {
				t.Fatal(err)
			}

			if err = store.Replace(ctx, &test.newEvent); err != nil {
				t.Fatal(err)
			}

			switch test.replaced {
			case true:
				// check newEvent has been saved
				var event nostr.Event
				row := store.DB.QueryRowContext(ctx, "SELECT * FROM events WHERE id = ?", test.newEvent.ID)
				err = row.Scan(&event.ID, &event.PubKey, &event.CreatedAt, &event.Kind, &event.Tags, &event.Content, &event.Sig)

				if err != nil {
					t.Fatalf("newEvent was not saved: %v", err)
				}

				if !reflect.DeepEqual(event, test.newEvent) {
					t.Fatalf("newEvent was not saved correctly.\n original %v, got %v", test.newEvent, event)
				}

				// check storedEvent has been deleted
				var rowsCount int
				if err = store.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM events WHERE id = ?", test.storedEvent.ID).Scan(&rowsCount); err != nil {
					t.Fatalf("failed to count the rows in the events table: %v", err)
				}

				if rowsCount != 0 {
					t.Fatal("storedEvent has not been deleted")
				}

			case false:
				// check storedEvent has NOT been altered
				var event nostr.Event
				row := store.DB.QueryRowContext(ctx, `SELECT * FROM events WHERE id = ?`, test.storedEvent.ID)
				err = row.Scan(&event.ID, &event.PubKey, &event.CreatedAt, &event.Kind, &event.Tags, &event.Content, &event.Sig)

				if err != nil {
					t.Fatalf("failed to query for storedEvent: %v", err)
				}

				if !reflect.DeepEqual(event, test.storedEvent) {
					t.Fatalf("storedEvent has been altered.\n original %v, got %v", test.storedEvent, event)
				}

				// check newEvent has NOT been stored
				var rowsCount int
				if err = store.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM events WHERE id = ?", test.newEvent.ID).Scan(&rowsCount); err != nil {
					t.Fatalf("failed to count the rows in the events table: %v", err)
				}

				if rowsCount != 0 {
					t.Fatal("newEvent should not have been stored")
				}
			}
		})
	}
}

// This test can fail because iterating over the filter.Tags (nostr.TagMap)
// doesn't preserve the order. Just run it again.
func TestBuildQuery(t *testing.T) {
	until := nostr.Timestamp(100)
	since := nostr.Timestamp(1000)

	filter := nostr.Filter{
		IDs:     []string{"a", "b", "c"},
		Kinds:   []int{0, 1, 2, 3, 4},
		Authors: []string{"pip", "calle", "fran"},
		Tags: nostr.TagMap{
			"e": {"id1", "id2", "id3"},
			"p": {"pk1", "pk2"},
		},
		Until: &until,
		Since: &since,
		Limit: 69,
	}

	expectedQuery := "SELECT id, pubkey, created_at, kind, tags, content, sig FROM events WHERE id IN (?,?,?) AND kind IN (?,?,?,?,?) AND pubkey IN (?,?,?) AND created_at <= ? AND created_at >= ? AND ( EXISTS ( SELECT 1 FROM json_each(tags, '$.e') WHERE json_each.value IN (?,?,?) ) OR EXISTS ( SELECT 1 FROM json_each(tags, '$.p') WHERE json_each.value IN (?,?) ) ) ORDER BY created_at DESC, id LIMIT ?"
	expectedArgs := []any{"a", "b", "c", 0, 1, 2, 3, 4, "pip", "calle", "fran", until.Time().Unix(), since.Time().Unix(), "id1", "id2", "id3", "pk1", "pk2", 69}
	query, args := buildQuery(filter)

	if query != expectedQuery {
		t.Errorf("expected query %v,\n got %v", expectedQuery, query)
	}

	if !reflect.DeepEqual(args, expectedArgs) {
		t.Errorf("expected args %v,\n got %v", expectedArgs, args)
	}
}
