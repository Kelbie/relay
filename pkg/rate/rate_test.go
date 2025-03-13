package rate

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/vertex-lab/crawler/pkg/database/redisdb"
	"github.com/vertex-lab/crawler/pkg/store/redistore"
	"github.com/vertex-lab/crawler/pkg/utils/redisutils"
)

type pubkeyData struct {
	pubkey string
	ID     string
	walks  int
	Bucket
}

var (
	reputableKeyWithBucket    = pubkeyData{pubkey: "reputable1", ID: "0", Bucket: Bucket{Tokens: 100, LastModified: 420}, walks: DefaultWalksThreshold}
	reputableKeyWithoutBucket = pubkeyData{pubkey: "reputable2", ID: "1", walks: DefaultWalksThreshold}
	lowReputationKey          = pubkeyData{pubkey: "spammer1", ID: "2"}
	unknownKey                = pubkeyData{pubkey: "spammer2"}
)

func TestBucket(t *testing.T) {
	redis := redisutils.SetupTestClient()
	defer redisutils.CleanupRedis(redis)

	bucket := Bucket{Tokens: 69, LastModified: 420}
	if err := addBucket(redis, "reputable", bucket); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	tests := []struct {
		name           string
		pubkey         string
		expectedBucket *Bucket
	}{
		{
			name:           "bucket not found",
			pubkey:         "random",
			expectedBucket: &Bucket{},
		},
		{
			name:           "bucket found",
			pubkey:         "reputable",
			expectedBucket: &bucket,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			limiter := NewLimiter(redis)
			bucket, err := limiter.Bucket(test.pubkey)
			if err != nil {
				t.Fatalf("expected error nil, got %v", err)
			}

			if !reflect.DeepEqual(bucket, test.expectedBucket) {
				t.Fatalf("expected bucket %v, got %v", test.expectedBucket, bucket)
			}
		})
	}
}

// We simulate running the Lua function automatic_refill() by calling the pay()
// function with a cost of 0.
func TestAutomaticRefill(t *testing.T) {
	redis := redisutils.SetupTestClient()
	defer redisutils.CleanupRedis(redis)

	now := time.Now().Unix()
	tests := []struct {
		name           string
		pubkeyData     pubkeyData
		policy         PagerankRefillPolicy
		expectedBucket *Bucket
	}{
		{
			name:           "too many tokens, no refill",
			pubkeyData:     reputableKeyWithBucket,
			policy:         PagerankRefillPolicy{MaxTokensBeforeRefill: 0},
			expectedBucket: &Bucket{Tokens: 100, LastModified: now},
		},
		{
			name:           "not enough time passed, no refill",
			pubkeyData:     reputableKeyWithBucket,
			policy:         PagerankRefillPolicy{RefillIntervalSeconds: math.MaxInt},
			expectedBucket: &Bucket{Tokens: 100, LastModified: now},
		},
		{
			name:           "unknown key, no refill",
			pubkeyData:     unknownKey,
			policy:         NewPagerankRefillPolicy(),
			expectedBucket: &Bucket{LastModified: now},
		},
		{
			name:           "low reputation key, no refill",
			pubkeyData:     lowReputationKey,
			policy:         NewPagerankRefillPolicy(),
			expectedBucket: &Bucket{LastModified: now},
		},
		{
			name:           "reputable without bucket, refill",
			pubkeyData:     reputableKeyWithoutBucket,
			policy:         NewPagerankRefillPolicy(),
			expectedBucket: &Bucket{Tokens: DefaultRefillTokens, LastModified: now},
		},
		{
			name:           "reputable with bucket, refill",
			pubkeyData:     reputableKeyWithBucket,
			policy:         NewPagerankRefillPolicy(),
			expectedBucket: &Bucket{Tokens: 100 + DefaultRefillTokens, LastModified: now},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			limiter := Limiter{
				client: redis,
				policy: test.policy,
			}

			if err := setup(redis, test.pubkeyData); err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			_, err := limiter.Pay(test.pubkeyData.pubkey, 0)
			if err != nil {
				t.Fatalf("expected error nil, got %v", err)
			}

			bucket, err := limiter.Bucket(test.pubkeyData.pubkey)
			if err != nil {
				t.Fatalf("expected error nil, got %v", err)
			}

			if !reflect.DeepEqual(bucket, test.expectedBucket) {
				t.Fatalf("expected bucket %v, got %v", test.expectedBucket, bucket)
			}
		})
	}
}

// We simulate running the Lua function pay() without automatic refills by using
// the policy maxTokensBeforeRefill = 0
func TestPay(t *testing.T) {
	redis := redisutils.SetupTestClient()
	defer redisutils.CleanupRedis(redis)

	tests := []struct {
		name       string
		pubkeyData pubkeyData
		cost       int
		policy     PagerankRefillPolicy

		expectedPaid   bool
		expectedBucket *Bucket
	}{
		{
			name:       "not enough tokens",
			pubkeyData: reputableKeyWithBucket,
			cost:       101,
			policy:     PagerankRefillPolicy{MaxTokensBeforeRefill: 0},

			expectedPaid:   false,
			expectedBucket: &reputableKeyWithBucket.Bucket,
		},
		{
			name:       "enough tokens",
			pubkeyData: reputableKeyWithBucket,
			cost:       10,
			policy:     PagerankRefillPolicy{MaxTokensBeforeRefill: 0},

			expectedPaid:   true,
			expectedBucket: &Bucket{Tokens: 90, LastModified: time.Now().Unix()},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			limiter := Limiter{
				client: redis,
				policy: test.policy,
			}

			if err := setup(redis, test.pubkeyData); err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			paid, err := limiter.Pay(test.pubkeyData.pubkey, test.cost)
			if err != nil {
				t.Fatalf("expected error nil, got %v", err)
			}

			if paid != test.expectedPaid {
				t.Fatalf("expected paid %v, got %v", test.expectedPaid, paid)
			}

			bucket, err := limiter.Bucket(test.pubkeyData.pubkey)
			if err != nil {
				t.Fatalf("expected error nil, got %v", err)
			}

			if !reflect.DeepEqual(bucket, test.expectedBucket) {
				t.Fatalf("expected bucket %v, got %v", test.expectedBucket, bucket)
			}
		})
	}
}

func TestTopUp(t *testing.T) {
	redis := redisutils.SetupTestClient()
	defer redisutils.CleanupRedis(redis)

	limiter := NewLimiter(redis)
	_, err := limiter.TopUp("whatever", 100)
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

func BenchmarkPay(b *testing.B) {
	redis := redisutils.SetupTestClient()
	defer redisutils.CleanupRedis(redis)

	limiter := NewLimiter(redis)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pubkey := strconv.FormatInt(int64(i), 10)
		if _, err := limiter.Pay(pubkey, 1); err != nil {
			b.Fatalf("bechmark failed: %v", err)
		}
	}
}

// -----------------------------------HELPERS----------------------------------

// setup redis with the pubkeyData, adding keys walks, and buckets.
func setup(redis *redis.Client, data pubkeyData) error {
	if err := addKey(redis, data.pubkey, data.ID); err != nil {
		return err
	}

	if err := addWalks(redis, data.ID, data.walks); err != nil {
		return err
	}

	if err := addBucket(redis, data.pubkey, data.Bucket); err != nil {
		return err
	}

	return nil
}

// adds key to the keyIndex
func addKey(redis *redis.Client, pubkey, ID string) error {
	if err := redis.HSet(context.Background(), redisdb.KeyKeyIndex, pubkey, ID).Err(); err != nil {
		return fmt.Errorf("failed to add pubkey %s to the keyIndex: %w", pubkey, err)
	}

	return nil
}

// adds walks to walksVisiting:`ID` if walks is greater than zero.
func addWalks(redis *redis.Client, ID string, walks int) error {
	if walks < 1 {
		return nil
	}

	walksToAdd := make([]any, walks)
	for i := 0; i < walks; i++ {
		walksToAdd[i] = i
	}

	set := redistore.KeyWalksVisitingPrefix + ID
	if err := redis.SAdd(context.Background(), set, walksToAdd...).Err(); err != nil {
		return fmt.Errorf("failed to add walks to %s: %w", set, err)
	}

	return nil
}

// adds bucket to a pubkey is the bucket is not empty
func addBucket(redis *redis.Client, pubkey string, bucket Bucket) error {
	if bucket.Tokens == 0 && bucket.LastModified == 0 {
		return nil
	}

	bucketKey := "creditBucket:" + pubkey
	err := redis.HSet(context.Background(), bucketKey, "tokens", bucket.Tokens, "last_modified", bucket.LastModified).Err()
	if err != nil {
		return fmt.Errorf("failed to set %s: %w", bucketKey, err)
	}

	return nil
}
