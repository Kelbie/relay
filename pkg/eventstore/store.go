// The package eventstore defines the Store struct which expose simple methods to Save, Delete, Replace,
// nostr.Event and Query the SQLite3 databse using a nostr.Filter. Inspired by fiatjaf's eventstore
package eventstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/nbd-wtf/go-nostr"
)

var (
	ErrFilterIsNil    = errors.New("filter is nil")
	ErrTooManyIDs     = errors.New("too many IDs in filter")
	ErrTooManyAuthors = errors.New("too many authors in filter")
	ErrTooManyKinds   = errors.New("too many kinds in filter")
	ErrTooManyTags    = errors.New("too many tags in filter")
	ErrEmptyFilter    = errors.New("filter must specify at least one ID, kind, author, tag, or time range")

	// this is used to identify internal query errors, and should not be returned to the client that sent a filter
	ErrInternalQuery = errors.New("internal query error")
)

type QueryLimits struct {
	maxIDs     int
	maxKinds   int
	maxAuthors int
	maxTags    int

	defaultLimit int
	maxLimit     int
}

// NewQueryLimits() returns a default query limits struct.
func NewQueryLimits() QueryLimits {
	return QueryLimits{
		maxIDs:     100,
		maxKinds:   10,
		maxAuthors: 100,
		maxTags:    5,

		defaultLimit: 10,
		maxLimit:     50, // the maximum number of events returned per query (using a filter)
	}
}

type Store struct {
	*sql.DB
	QueryLimits
}

// Profile represent the internal representation of the content of kind0s, used for full-text-search.
type Profile struct {
	ID          string
	Pubkey      string
	Name        string
	DisplayName string
	About       string
	Website     string
	Nip05       string
}

var schema = `CREATE TABLE IF NOT EXISTS events (
       id TEXT PRIMARY KEY,
       pubkey TEXT NOT NULL,
       created_at INTEGER NOT NULL,
       kind INTEGER NOT NULL,
       tags JSONB NOT NULL,
       content TEXT NOT NULL,
       sig TEXT NOT NULL);

	CREATE INDEX IF NOT EXISTS pubkey_idx ON events(pubkey);
	CREATE INDEX IF NOT EXISTS time_idx ON events(created_at DESC);
	CREATE INDEX IF NOT EXISTS kind_idx ON events(kind);

	CREATE VIRTUAL TABLE IF NOT EXISTS profiles_fts USING fts5(
		id UNINDEXED,
		pubkey UNINDEXED,
		name,
		display_name,
		about,
		website,
		nip05,
		tokenize = 'trigram',
	  );

	CREATE TRIGGER IF NOT EXISTS profiles_ai AFTER INSERT ON events
	  WHEN NEW.kind = 0
	  BEGIN
	  INSERT INTO profiles_fts (id, pubkey, name, display_name, about, website, nip05)
	  VALUES (
	  	NEW.id,
	  	NEW.pubkey,
	  	NEW.content ->> '$.name',
	  	COALESCE( NEW.content ->> '$.display_name', NEW.content ->> '$.displayName'),
	  	NEW.content ->> '$.about',
	  	NEW.content ->> '$.website',
	  	NEW.content ->> '$.nip05'
	  );
	  END;

	CREATE TRIGGER IF NOT EXISTS profiles_ad AFTER DELETE ON events
	WHEN OLD.kind = 0
	BEGIN
	  DELETE FROM profiles_fts WHERE id = OLD.id;
	END;`

// New() returns a new Store with default QueryLimits.
func New(DatabaseURL string) (*Store, error) {
	var s = &Store{QueryLimits: NewQueryLimits()}
	var err error

	s.DB, err = sql.Open("sqlite3", DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to sqlite3 %s: %w", DatabaseURL, err)
	}

	if _, err := s.DB.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to apply data schema: %w", err)
	}

	return s, nil
}

// Save() saves the specified event. For replaceable events, it's recommended to use Replace().
func (s *Store) Save(ctx context.Context, event *nostr.Event) error {
	tags, err := json.Marshal(event.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal the tags: %w", err)
	}

	_, err = s.DB.ExecContext(ctx, `INSERT OR IGNORE INTO events (id, pubkey, created_at, kind, tags, content, sig)
        VALUES ($1, $2, $3, $4, $5, $6, $7)`, event.ID, event.PubKey, event.CreatedAt, event.Kind, tags, event.Content, event.Sig)

	if err != nil {
		return fmt.Errorf("failed to save event with ID %s: %w", event.ID, err)
	}

	return nil
}

// Delete() deletes the event whose ID matches the specified eventID.
func (s *Store) Delete(ctx context.Context, eventID string) error {
	if _, err := s.DB.ExecContext(ctx, "DELETE FROM events WHERE id = $1", eventID); err != nil {
		return fmt.Errorf("failed to delete event with ID %s: %w", eventID, err)
	}

	return nil
}

// Replace() is meant to be used for replaceable events. If that's not the case, use Save().
// - if there are no stored events with kind = event.Kind, pubkey = event.PubKey, then saves `event`.
// - if there is a stored event with kind = event.Kind, pubkey = event.PubKey, replace it with `event` only if the latter is strictly newer.
func (s *Store) Replace(ctx context.Context, event *nostr.Event) error {
	var oldID string
	var oldCreatedAt nostr.Timestamp
	row := s.DB.QueryRowContext(ctx, "SELECT id, created_at FROM events WHERE kind = $1 AND pubkey = $2", event.Kind, event.PubKey)
	err := row.Scan(&oldID, &oldCreatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		// if there are no events for that kind for that pubkey, then save
		return s.Save(ctx, event)
	}

	if err != nil {
		return fmt.Errorf("failed to query for old events: %w", err)
	}

	if oldCreatedAt >= event.CreatedAt {
		// if the event is not newer, then don't replace
		return nil
	}

	tags, err := json.Marshal(event.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal the tags: %w", err)
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to initiate the transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO events (id, pubkey, created_at, kind, tags, content, sig)
	VALUES ($1, $2, $3, $4, $5, $6, $7)`, event.ID, event.PubKey, event.CreatedAt, event.Kind, tags, event.Content, event.Sig)

	if err != nil {
		return fmt.Errorf("failed to save event with ID %s: %w", event.ID, err)
	}

	if _, err = tx.ExecContext(ctx, "DELETE FROM events WHERE id = $1", oldID); err != nil {
		return fmt.Errorf("failed to delete old event with ID %s: %w", oldID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to replace event %s with event %s: %w", oldID, event.ID, err)
	}

	return nil
}

// Query() queries the database using the provided nostr.Filter.
// The error it returns is part of one of these three categories:
//
// - the filter is invalid (e.g. too many IDs)
//
// - the query failed
//
// - scanning a row into an event failed
//
// In the first two cases, the error should be returned to the client and logged.
// In the third case, the error should only be logged.
func (s *Store) Query(ctx context.Context, filter *nostr.Filter) ([]nostr.Event, error) {
	if err := s.validate(filter); err != nil {
		return nil, err
	}

	query, args := buildQuery(filter)
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to fetch events with query %s: %w", query, err)
	}
	defer rows.Close()

	var events []nostr.Event
	for rows.Next() {
		var event nostr.Event
		err = rows.Scan(&event.ID, &event.PubKey, &event.CreatedAt, &event.Kind, &event.Tags, &event.Content, &event.Sig)

		if err != nil {
			// if an error occurs during the scan, it means we have problems with our database.
			// we should return the events found so far to the client, but no error, which should just be logged.
			return events, fmt.Errorf("%w: failed to scan event row: %w", ErrInternalQuery, err)
		}

		events = append(events, event)
	}

	return events, nil
}

func (s *Store) validate(filter *nostr.Filter) error {
	if filter == nil {
		return ErrFilterIsNil
	}

	IDs := len(filter.IDs)
	if IDs > s.maxIDs {
		return fmt.Errorf("%w: max %d, requested %d", ErrTooManyIDs, s.maxIDs, IDs)
	}

	kinds := len(filter.Kinds)
	if kinds > s.maxKinds {
		return fmt.Errorf("%w: max %d, requested %d", ErrTooManyKinds, s.maxKinds, kinds)
	}

	authors := len(filter.Authors)
	if authors > s.maxAuthors {
		return fmt.Errorf("%w: max %d, requested %d", ErrTooManyAuthors, s.maxAuthors, authors)
	}

	tags := len(filter.Tags)
	if tags > s.maxTags {
		return fmt.Errorf("%w: max %d, requested %d", ErrTooManyTags, s.maxTags, tags)
	}

	if IDs+kinds+authors+tags == 0 && filter.Since == nil && filter.Until == nil {
		return ErrEmptyFilter
	}

	if filter.Limit > s.maxLimit {
		// overwrite the limit with the maximum allowed
		filter.Limit = s.maxLimit
	}

	if filter.Limit < 1 {
		// overwrite the limit with the default value
		filter.Limit = s.defaultLimit
	}

	return nil
}

// The function buildQuery translates a nostr.Filter into a SQL query and a list
// of arguments filling the '?' of the former.
func buildQuery(filter *nostr.Filter) (string, []any) {
	var conditions []string
	var args []any

	if len(filter.IDs) > 0 {
		conditions = append(conditions, "id IN "+ValueList(len(filter.IDs)))
		for _, ID := range filter.IDs {
			args = append(args, ID)
		}
	}

	if len(filter.Kinds) > 0 {
		conditions = append(conditions, "kind IN "+ValueList(len(filter.Kinds)))
		for _, kind := range filter.Kinds {
			args = append(args, kind)
		}
	}

	if len(filter.Authors) > 0 {
		conditions = append(conditions, "pubkey IN "+ValueList(len(filter.Authors)))
		for _, author := range filter.Authors {
			args = append(args, author)
		}
	}

	if filter.Until != nil {
		conditions = append(conditions, "created_at <= ?")
		args = append(args, filter.Until.Time().Unix())
	}

	if filter.Since != nil {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, filter.Since.Time().Unix())
	}

	if len(filter.Tags) > 0 {
		tagCond := make([]string, 0, len(filter.Tags))
		for key, vals := range filter.Tags {
			if len(vals) == 0 {
				continue
			}

			tagCond = append(tagCond,
				`EXISTS (
					SELECT 1 FROM json_each(tags) 
					WHERE json_each.value ->> 0 = ? 
					AND json_each.value ->> 1 IN `+ValueList(len(vals))+
					` )`)

			args = append(args, key)
			for _, val := range vals {
				args = append(args, val)
			}
		}

		// tag conditions are OR-ed together
		conditions = append(conditions, "( "+strings.Join(tagCond, " OR ")+" )")
	}

	args = append(args, filter.Limit)
	query := "SELECT id, pubkey, created_at, kind, tags, content, sig FROM events WHERE " +
		strings.Join(conditions, " AND ") + " ORDER BY created_at DESC, id LIMIT ?"

	return query, args
}

// ValueList() returns n question marks separated by commas and
// enclosed by parenthesis for SQL value lists in queries. e.g. ValueList(4) = "(?,?,?,?)".
//
// WARNING: It panics if n < 1.
func ValueList(n int) string {
	return "(?" + strings.Repeat(",?", n-1) + ")"
}
