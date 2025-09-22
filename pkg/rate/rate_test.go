package rate

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/vertex-lab/crawler_v2/pkg/redb"
)

type user struct {
	pubkey      string
	isReputable bool
	Bucket
}

var (
	ctx         = context.Background()
	testAddress = "localhost:6380"
	Sep2025     = int64(1758047781)

	reputableFull     = user{pubkey: "reputable1", isReputable: true, Bucket: Bucket{Tokens: DefaultAmount, LastModified: Sep2025}}
	reputableToRefill = user{pubkey: "reputable2", isReputable: true, Bucket: Bucket{Tokens: DefaultAmount / 2, LastModified: Sep2025}}
	reputableEmpty    = user{pubkey: "reputable3", isReputable: true}
	spammer           = user{pubkey: "spammer1"}
	unknown           = user{pubkey: "spammer2"}
	users             = []user{reputableFull, reputableToRefill, reputableEmpty, spammer, unknown}
)

// setup redis with the user, adding keys walks, and buckets.
func setup(db redb.RedisDB) error {
	ctx := context.Background()

	for i, user := range users {
		if _, err := db.AddNode(ctx, user.pubkey); err != nil {
			return fmt.Errorf("setup failed: %w", err)
		}

		err := db.Client.HSet(ctx, creditBucket(user.pubkey), KeyTokens, user.Tokens, KeyLastModified, user.LastModified).Err()
		if err != nil {
			return fmt.Errorf("setup failed: %w", err)
		}

		if user.isReputable {
			walks := make([]any, 100)
			for i := range 100 {
				walks[i] = i
			}

			key := redb.KeyWalksVisitingPrefix + strconv.Itoa(i)
			if err := db.Client.SAdd(ctx, key, walks...).Err(); err != nil {
				return fmt.Errorf("setup failed: %w", err)
			}
		}
	}

	return nil
}

// We simulate running the Lua function "automatic_refill" by calling [Allow] with a cost of 0.
func TestAutomaticRefill(t *testing.T) {
	db := redb.New(&redis.Options{Addr: testAddress})
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
			refill: RefillPolicy{Amount: 0},
			bucket: reputableFull.Bucket,
		},
		{
			name:   "too soon, no refill",
			user:   reputableToRefill,
			refill: RefillPolicy{Interval: 7 * 24 * time.Hour},
			bucket: reputableToRefill.Bucket,
		},
		{
			name:   "unknown key, no refill",
			user:   unknown,
			refill: NewRefillPolicy(),
			bucket: unknown.Bucket,
		},
		{
			name:   "low reputation key, no refill",
			user:   spammer,
			refill: NewRefillPolicy(),
			bucket: spammer.Bucket,
		},
		{
			name:   "reputable without bucket, refill",
			user:   reputableEmpty,
			refill: NewRefillPolicy(),
			bucket: Bucket{Tokens: DefaultAmount, LastModified: now},
		},
		{
			name:   "reputable with bucket, refill",
			user:   reputableToRefill,
			refill: NewRefillPolicy(),
			bucket: Bucket{Tokens: DefaultAmount, LastModified: now},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			limiter, err := NewLimiter(db.Client, test.refill)
			if err != nil {
				t.Fatal(err)
			}

			allow := limiter.Allow(test.user.pubkey, 0)
			if !allow {
				t.Fatalf("failed to allow a cost of 0")
			}

			bucket, err := limiter.Bucket(test.user.pubkey)
			if err != nil {
				t.Fatalf("expected error nil, got %v", err)
			}

			if !reflect.DeepEqual(bucket, test.bucket) {
				t.Fatalf("expected bucket %v, got %v", test.bucket, bucket)
			}
		})
	}
}

// We simulate running the Lua function "pay" without "automatic_refill" by using
// the policy maxTokensBeforeRefill = 0
func TestAllow(t *testing.T) {
	db := redb.New(&redis.Options{Addr: testAddress})
	defer db.Client.FlushAll(context.Background())

	if err := setup(db); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		user   user
		cost   int
		allow  bool
		bucket Bucket
	}{
		{
			name:   "not enough tokens",
			user:   reputableFull,
			cost:   DefaultAmount + 1,
			allow:  false,
			bucket: reputableFull.Bucket,
		},
		{
			name:   "enough tokens",
			user:   reputableFull,
			cost:   DefaultAmount - 1,
			allow:  true,
			bucket: Bucket{Tokens: 1, LastModified: time.Now().Unix()},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			limiter, err := NewLimiter(db.Client, NoRefill)
			if err != nil {
				t.Fatal(err)
			}

			allow := limiter.Allow(test.user.pubkey, test.cost)
			if allow != test.allow {
				t.Fatalf("expected %v, got %v", test.allow, allow)
			}

			bucket, err := limiter.Bucket(test.user.pubkey)
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

	limiter, err := NewLimiter(db, NoRefill)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	_, err = limiter.TopUp("whatever", 100)
	if err != nil {
		t.Fatalf("expected error nil, got %v", err)
	}

	total, err := limiter.TopUp("whatever", 69)
	if err != nil {
		t.Fatalf("expected error nil, got %v", err)
	}

	if total != 169 {
		t.Fatalf("expected total 169, got %d", total)
	}

	bucket, err := limiter.Bucket("whatever")
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

	limiter, err := NewLimiter(db, NoRefill)
	if err != nil {
		b.Fatalf("expected nil, got %v", err)
	}

	_, err = limiter.TopUp("pubkey", 100_000)
	if err != nil {
		b.Fatalf("expected nil, got %v", err)
	}

	b.ResetTimer()
	for range b.N {
		limiter.Allow("pubkey", 1)
	}
}
