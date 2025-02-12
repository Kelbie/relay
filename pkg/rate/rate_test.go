package rate

import (
	"testing"
	"time"
)

func TestBucket(t *testing.T) {
	testCases := []struct {
		name           string
		bucket         *Bucket
		cost           int
		expectedReject bool
	}{
		{
			name:           "nil bucket",
			expectedReject: true,
		},
		{
			name: "not enough tokens",
			bucket: &Bucket{
				tokens:  10,
				lastReq: time.Now(),
			},
			cost:           69,
			expectedReject: true,
		},
		{
			name: "enough tokens",
			bucket: &Bucket{
				tokens:  10,
				lastReq: time.Now(),
			},
			cost:           2,
			expectedReject: false,
		},
		{
			name: "enough tokens thanks to refill",
			bucket: &Bucket{
				tokens:  10,
				lastReq: time.Now().Add(-DefaultInterval),
			},
			cost:           DefaultFreeTokens,
			expectedReject: false,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			reject, _ := test.bucket.Reject(test.cost)
			if reject != test.expectedReject {
				t.Errorf("expected %v, got %v", test.expectedReject, reject)
			}
		})
	}
}
