package notifydata_test

import (
	"testing"

	"github.com/blinkdisk/core/notification/notifydata"
)

func TestEmptyEventInfo(t *testing.T) {
	testRoundTrip(t, &notifydata.EmptyEventData{})
}
