package credits

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/vertex-lab/crawler_v2/pkg/regraph"
)

type user struct {
	pubkey   string
	addition int64 // unix time
	walks    int
	Bucket
}

var (
	ctx         = context.Background()
	testAddress = "localhost:6380"
	Sep2025     = int64(1758047781) // unix time

	reputableFull = user{
		pubkey:   "reputable1",
		addition: Sep2025,
		walks:    100,
		Bucket:   Bucket{Tokens: 100, LastModified: Sep2025},
	}

	reputableEmpty = user{
		pubkey:   "reputable2",
		addition: Sep2025,
		walks:    100,
	}

	reputableTooYoung = user{
		pubkey:   "reputable3",
		addition: time.Now().Unix(),
		walks:    100,
	}

	spammer = user{pubkey: "spammer1"}
	unknown = user{pubkey: "spammer2"}

	users = []user{
		reputableFull,
		reputableEmpty,
		reputableTooYoung,
		spammer,
		unknown,
	}
)

// setup redis with the user, adding keys walks, and buckets.
func setup(db regraph.DB) error {
	ctx := context.Background()

	for i, user := range users {
		id := strconv.Itoa(i)

		if _, err := db.AddNode(ctx, user.pubkey); err != nil {
			return fmt.Errorf("setup failed: %w", err)
		}

		// changing addition timestamp with the provided one
		if err := db.Client.HSet(ctx, regraph.KeyNodePrefix+id, regraph.NodeAddedTS, user.addition).Err(); err != nil {
			return fmt.Errorf("setup failed: %w", err)
		}

		err := db.Client.HSet(ctx, creditBucket(user.pubkey), KeyTokens, user.Tokens, KeyLastModified, user.LastModified).Err()
		if err != nil {
			return fmt.Errorf("setup failed: %w", err)
		}

		if user.walks > 0 {
			walks := make([]any, user.walks)
			for i := range user.walks {
				walks[i] = i
			}

			key := regraph.KeyWalksVisitingPrefix + id
			if err := db.Client.SAdd(ctx, key, walks...).Err(); err != nil {
				return fmt.Errorf("setup failed: %w", err)
			}
		}
	}

	return nil
}

// We simulate running the Lua function "automatic_refill" by calling [Deduct] with a cost of 0.
func TestAutomaticRefill(t *testing.T) {
	db, err := regraph.New(&redis.Options{Addr: testAddress})
	if err != nil {
		t.Fatalf("setup failed %v", err)
	}
	defer db.Client.FlushAll(ctx)

	if err := setup(db); err != nil {
		t.Fatal(err)
	}

	now := time.Now().Unix()
	tests := []struct {
		name   string
		user   user
		refill RefillPolicy
		bucket Bucket
	}{
		{
			name:   "too many tokens, no refill",
			user:   reputableFull,
			refill: NewRefillPolicy(),
			bucket: reputableFull.Bucket,
		},
		{
			name:   "too soon, no refill",
			user:   reputableEmpty,
			refill: RefillPolicy{Interval: math.MaxInt64},
			bucket: reputableEmpty.Bucket,
		},
		{
			name:   "key too young, no refill",
			user:   reputableTooYoung,
			refill: NewRefillPolicy(),
			bucket: reputableTooYoung.Bucket,
		},
		{
			name:   "unknown key, no refill",
			user:   unknown,
			refill: NewRefillPolicy(),
			bucket: Bucket{},
		},
		{
			name:   "low reputation key, no refill",
			user:   spammer,
			refill: NewRefillPolicy(),
			bucket: Bucket{},
		},
		{
			name:   "reputable, refill",
			user:   reputableEmpty,
			refill: NewRefillPolicy(),
			bucket: Bucket{Tokens: 100, LastModified: now},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manager, err := NewManager(db.Client, test.refill)
			if err != nil {
				t.Fatal(err)
			}

			err = manager.Deduct(test.user.pubkey, 0)
			if err != nil {
				t.Fatalf("failed to deduct a cost of 0")
			}

			bucket, err := manager.Bucket(test.user.pubkey)
			if err != nil {
				t.Fatalf("expected error nil, got %v", err)
			}

			if !reflect.DeepEqual(bucket, test.bucket) {
				t.Fatalf("expected bucket %v, got %v", test.bucket, bucket)
			}
		})
	}
}

// We simulate running the Lua function "deduct" without "automatic_refill" by using [NoRefill].
func TestDeduct(t *testing.T) {
	db, err := regraph.New(&redis.Options{Addr: testAddress})
	if err != nil {
		t.Fatalf("setup failed %v", err)
	}
	defer db.Client.FlushAll(context.Background())

	if err := setup(db); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		user   user
		cost   int
		err    error
		bucket Bucket
	}{
		{
			name:   "not enough tokens",
			user:   reputableFull,
			cost:   101,
			err:    ErrInsufficientCredits,
			bucket: reputableFull.Bucket,
		},
		{
			name:   "enough tokens",
			user:   reputableFull,
			cost:   99,
			bucket: Bucket{Tokens: 1, LastModified: time.Now().Unix()},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manager, err := NewManager(db.Client, NoRefill)
			if err != nil {
				t.Fatal(err)
			}

			err = manager.Deduct(test.user.pubkey, test.cost)
			if !errors.Is(err, test.err) {
				t.Fatalf("expected error %v, got %v", test.err, err)
			}

			bucket, err := manager.Bucket(test.user.pubkey)
			if err != nil {
				t.Fatalf("expected error nil, got %v", err)
			}

			if !reflect.DeepEqual(bucket, test.bucket) {
				t.Fatalf("expected bucket %v, got %v", test.bucket, bucket)
			}
		})
	}
}

func TestTopUp(t *testing.T) {
	db := redis.NewClient(&redis.Options{Addr: testAddress})
	defer db.FlushAll(context.Background())

	manager, err := NewManager(db, NoRefill)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	_, err = manager.TopUp("whatever", 100)
	if err != nil {
		t.Fatalf("expected error nil, got %v", err)
	}

	total, err := manager.TopUp("whatever", 69)
	if err != nil {
		t.Fatalf("expected error nil, got %v", err)
	}

	if total != 169 {
		t.Fatalf("expected total 169, got %d", total)
	}

	bucket, err := manager.Bucket("whatever")
	if err != nil {
		t.Fatalf("expected error nil, got %v", err)
	}

	if bucket.Tokens != 169 {
		t.Fatalf("expected bucket's tokens 169, got %d", bucket.Tokens)
	}
}

// ---------------------------------BENCHMARKS---------------------------------

func BenchmarkAllow(b *testing.B) {
	db := redis.NewClient(&redis.Options{Addr: testAddress})
	defer db.FlushAll(context.Background())

	manager, err := NewManager(db, NoRefill)
	if err != nil {
		b.Fatalf("expected nil, got %v", err)
	}

	_, err = manager.TopUp("pubkey", 100_000)
	if err != nil {
		b.Fatalf("expected nil, got %v", err)
	}

	b.ResetTimer()
	for range b.N {
		manager.Deduct("pubkey", 1)
	}
}
