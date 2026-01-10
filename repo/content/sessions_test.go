package content

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/blinkdisk/core/internal/blobtesting"
	"github.com/blinkdisk/core/internal/clock"
	"github.com/blinkdisk/core/internal/epoch"
	"github.com/blinkdisk/core/internal/faketime"
	"github.com/blinkdisk/core/internal/testlogging"
	"github.com/blinkdisk/core/repo/blob"
	"github.com/blinkdisk/core/repo/content/index"
	"github.com/blinkdisk/core/repo/format"
)

func TestGenerateSessionID(t *testing.T) {
	n := clock.Now()

	s1, err := generateSessionID(n)
	require.NoError(t, err)

	s2, err := generateSessionID(n)
	require.NoError(t, err)

	s3, err := generateSessionID(n)
	require.NoError(t, err)

	m := map[SessionID]bool{
		s1: true,
		s2: true,
		s3: true,
	}

	if len(m) != 3 {
		t.Fatalf("session IDs were not unique: %v", m)
	}
}

func TestSessionIDFromBlobID(t *testing.T) {
	cases := []struct {
		blobID    blob.ID
		sessionID SessionID
	}{
		{"pdeadbeef", ""},
		{"pdeadbeef-", ""},
		{"pdeadbeef-whatever", ""},
		{"pdeadbeef-s01", "s01"},
		{"pdeadbeef-s01", "s01"},
		{"sdeadbeef-s01", "s01"},
	}

	for _, tc := range cases {
		if got, want := SessionIDFromBlobID(tc.blobID), tc.sessionID; got != want {
			t.Errorf("invalid result for %v: %v, want %v", tc.blobID, got, want)
		}
	}
}

func TestCheckClockSkewBounds_Positive(t *testing.T) {
	now := clock.Now()
	modTime := now.Add(maxClockSkew) // within maxClockSkew

	err := checkClockSkewBounds(now, modTime)
	require.NoError(t, err)
}

func TestCheckClockSkewBounds_Negative(t *testing.T) {
	now := clock.Now()
	modTime := now.Add(maxClockSkew + time.Nanosecond) // exceeds maxClockSkew

	err := checkClockSkewBounds(now, modTime)
	require.Error(t, err)
	require.Contains(t, err.Error(), "clock skew detected")
}

func TestMaybeCheckClockSkewBounds_Disabled(t *testing.T) {
	now := clock.Now()

	for _, tc := range []time.Time{
		now.Add(-maxClockSkew - time.Hour),
		now.Add(-maxClockSkew),
		now,
		now.Add(maxClockSkew),
		now.Add(maxClockSkew + 10*time.Hour),
	} {
		t.Run(tc.String(), func(t *testing.T) {
			// BLINKDISK_ENABLE_CLOCK_SKEW_CHECK is not set
			err := maybeCheckClockSkewBounds(now, tc)
			require.NoError(t, err)

			t.Setenv("BLINKDISK_ENABLE_CLOCK_SKEW_CHECK", "false")

			err = maybeCheckClockSkewBounds(now, tc)
			require.NoError(t, err)
		})
	}
}

func TestMaybeCheckClockSkewBounds_Enabled(t *testing.T) {
	t.Setenv("BLINKDISK_ENABLE_CLOCK_SKEW_CHECK", "true")

	now := clock.Now()

	for _, tc := range []time.Time{
		now.Add(-maxClockSkew - time.Hour),
		now.Add(maxClockSkew + 10*time.Hour),
	} {
		t.Run(tc.String(), func(t *testing.T) {
			err := maybeCheckClockSkewBounds(now, tc)
			require.Error(t, err)
		})
	}
}

func TestWriteSessionMarkerLockedWithoutClockSkew(t *testing.T) {
	t.Setenv("BLINKDISK_ENABLE_CLOCK_SKEW_CHECK", "1")

	ctx := testlogging.Context(t)
	data := blobtesting.DataMap{}
	keyTime := map[blob.ID]time.Time{}

	// Use TimeAdvance to control the timeNow() in the map storage.
	ta := faketime.NewTimeAdvance(time.Date(2018, time.January, 1, 0, 0, 0, 0, time.UTC))
	st := blobtesting.NewMapStorage(data, keyTime, ta.NowFunc())

	bm, err := NewManagerForTesting(testlogging.Context(t), st, mustCreateFormatProvider(t, &format.ContentFormat{
		Hash:       "HMAC-SHA256-128",
		Encryption: "AES256-GCM-HMAC-SHA256",
		HMACSecret: []byte("foo"),
		MasterKey:  []byte("0123456789abcdef0123456789abcdef"),
		MutableParameters: format.MutableParameters{
			Version:         2,
			MaxPackSize:     maxPackSize,
			IndexVersion:    index.Version2,
			EpochParameters: epoch.DefaultParameters(),
		},
	}), nil, &ManagerOptions{TimeNow: ta.NowFunc()}) // Use the same time advance for the manager's timeNow().
	require.NoError(t, err, "can't create bm")

	t.Cleanup(func() { bm.CloseShared(ctx) })

	err = bm.writeSessionMarkerLocked(ctx)
	require.NoError(t, err)
}

func TestWriteSessionMarkerLockedWithClockSkew(t *testing.T) {
	t.Setenv("BLINKDISK_ENABLE_CLOCK_SKEW_CHECK", "1")

	ctx := testlogging.Context(t)
	data := blobtesting.DataMap{}
	keyTime := map[blob.ID]time.Time{}

	bmTime := faketime.NewTimeAdvance(time.Date(2018, time.January, 1, 0, 0, 0, 0, time.UTC))
	stTime := faketime.NewTimeAdvance(bmTime.NowFunc()().Add(maxClockSkew + time.Nanosecond))
	st := blobtesting.NewMapStorage(data, keyTime, stTime.NowFunc())

	bm, err := NewManagerForTesting(testlogging.Context(t), st, mustCreateFormatProvider(t, &format.ContentFormat{
		Hash:       "HMAC-SHA256-128",
		Encryption: "AES256-GCM-HMAC-SHA256",
		HMACSecret: []byte("foo"),
		MasterKey:  []byte("0123456789abcdef0123456789abcdef"),
		MutableParameters: format.MutableParameters{
			Version:         2,
			MaxPackSize:     maxPackSize,
			IndexVersion:    index.Version2,
			EpochParameters: epoch.DefaultParameters(),
		},
	}), nil, &ManagerOptions{TimeNow: bmTime.NowFunc()})
	require.NoError(t, err, "can't create bm")

	t.Cleanup(func() { bm.CloseShared(ctx) })

	err = bm.writeSessionMarkerLocked(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "clock skew detected")
}
