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
	ErrTooManyIDs     = errors.New("too many IDs in filter")
	ErrTooManyAuthors = errors.New("too many authors in filter")
	ErrTooManyKinds   = errors.New("too many kinds in filter")
	ErrTooManyTags    = errors.New("too many tags in filter")
	ErrEmptyFilter    = errors.New("filter must specify at least one ID, kind, author, tag, or time range")
	ErrInvalidLimit   = errors.New("filter's limit must be strictly greater than zero")
)

type QueryLimits struct {
	maxIDs     int
	maxKinds   int
	maxAuthors int
	maxTags    int
	maxLimit   int
}

// NewQueryLimits() returns a default query limits struct.
func NewQueryLimits() QueryLimits {
	return QueryLimits{
		maxIDs:     500,
		maxKinds:   10,
		maxAuthors: 500,
		maxTags:    10,
		maxLimit:   100, // the maximum number of events returned per query (using a filter)
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
	DisplayName string `db:"display_name"`
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
	  	json_extract(NEW.content, '$.name'),
	  	COALESCE(json_extract(NEW.content, '$.display_name'), json_extract(NEW.content, '$.displayName')),
	  	json_extract(NEW.content, '$.about'),
	  	json_extract(NEW.content, '$.website'),
	  	json_extract(NEW.content, '$.nip05')
	  );
	  END;

	CREATE TRIGGER IF NOT EXISTS profiles_ad AFTER DELETE ON events
	WHEN OLD.kind = 0
	BEGIN
	  DELETE FROM profiles_fts WHERE id = OLD.id;
	END;`

// New() returns a new EventStore with default QueryLimits.
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

func (s *Store) validate(filter *nostr.Filter) error {
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

	if filter.Limit < 1 {
		return fmt.Errorf("%w: limit = %d", ErrInvalidLimit, filter.Limit)
	}

	if filter.Limit > s.maxLimit {
		// overwrite the limit in the filter
		filter.Limit = s.maxLimit
	}

	return nil
}

// The function valueList() returns n question marks separated by commas and
// enclosed by parenthesis for SQL value lists in queries. e.g. valueList(4) = "(?,?,?,?)"
func valueList(n int) string {
	if n < 1 {
		return ""
	}
	return "(?" + strings.Repeat(",?", n-1) + ")"
}

func buildQuery(filter nostr.Filter) (string, []any) {
	var conditions []string
	var params []any

	if len(filter.IDs) > 0 {
		conditions = append(conditions, "id IN "+valueList(len(filter.IDs)))
		for _, ID := range filter.IDs {
			params = append(params, ID)
		}
	}

	if len(filter.Kinds) > 0 {
		conditions = append(conditions, "kind IN "+valueList(len(filter.Kinds)))
		for _, kind := range filter.Kinds {
			params = append(params, kind)
		}
	}

	if len(filter.Authors) > 0 {
		conditions = append(conditions, "pubkey IN "+valueList(len(filter.Authors)))
		for _, author := range filter.Authors {
			params = append(params, author)
		}
	}

	if filter.Until != nil {
		conditions = append(conditions, "created_at <= ?")
		params = append(params, filter.Until.Time().Unix())
	}

	if filter.Since != nil {
		conditions = append(conditions, "created_at >= ?")
		params = append(params, filter.Since.Time().Unix())
	}

	if len(filter.Tags) > 0 {
		tagCond := make([]string, 0, len(filter.Tags))
		for key, vals := range filter.Tags {
			if len(vals) == 0 {
				continue
			}

			tagCond = append(tagCond, "json_extract(tags, '$."+key+"') IN "+valueList((len(vals))))
			for _, val := range vals {
				params = append(params, val)
			}
		}

		// tag conditions are OR-ed together
		conditions = append(conditions, "("+strings.Join(tagCond, " OR ")+")")
	}

	params = append(params, filter.Limit)
	query := "SELECT id, pubkey, created_at, kind, tags, content, sig FROM events WHERE " +
		strings.Join(conditions, " AND ") + " ORDER BY created_at DESC, id LIMIT ?"

	return query, params
}

func (s *Store) Query(ctx context.Context, filter nostr.Filter) ([]nostr.Event, error) {
	if err := s.validate(&filter); err != nil {
		return nil, err
	}

	return nil, nil
}
