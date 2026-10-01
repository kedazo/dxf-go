package dxf

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

func dublinOffset() (days float64) {
	return 2415020.0
}

func julianEpoch() time.Time {
	return time.Date(1899, 12, 31, 0, 0, 0, 0, time.UTC)
}

func boolFromShort(val int16) bool {
	return val != 0
}

func defaultIfEmpty(val, defaultValue string) string {
	if len(val) == 0 {
		return defaultValue
	}
	return val
}

func daysFromDuration(d time.Duration) (days float64) {
	return d.Hours() / 24.0
}

func julianDateFromTime(val time.Time) (julianDate float64) {
	if val.IsZero() {
		return 0
	}
	// seconds instead of a time.Duration, which overflows 292 years after the epoch
	seconds := float64(val.Unix()-julianEpoch().Unix()) + float64(val.Nanosecond())/1e9
	return dublinOffset() + seconds/(24*60*60)
}

// maxDurationDays is about the longest time.Duration.
const maxDurationDays = 106751.0

func durationFromDays(days float64) time.Duration {
	if !(math.Abs(days) < maxDurationDays) {
		// out of range for a time.Duration: treated as unset
		return 0
	}
	hours := days * 24.0
	minutes := hours * 60.0
	seconds := minutes * 60.0
	return time.Duration(int64(seconds)) * time.Second
}

func ensurePositiveOrDefault(val, defaultValue float64) float64 {
	if val < 0.0 {
		return defaultValue
	}
	return val
}

func handleFromString(val string) Handle {
	handle, err := strconv.ParseUint(strings.TrimSpace(val), 16, 64)
	if err != nil {
		return Handle(0)
	}
	return Handle(uint64(handle))
}

func shortFromBool(val bool) int16 {
	if val {
		return 1
	}
	return 0
}

func stringFromHandle(h Handle) string {
	return fmt.Sprintf("%X", uint64(h))
}

// maxJulianDays is 1 January 10000; later dates are as bogus as ones before the epoch.
const maxJulianDays = 5373484.5

func timeFromJulianDays(juliandDays float64) time.Time {
	if !(juliandDays >= dublinOffset() && juliandDays < maxJulianDays) {
		// unset (0) or out of range: the zero time, written back as 0
		return time.Time{}
	}
	// manualy adjust for 1s difference to make the AutoDesk specified date match:
	//   2451544.91568287 = 31 December 1999, 9:58:35PM
	correction := 1
	asSeconds := (juliandDays-dublinOffset())*24*60*60 + float64(correction)
	// seconds instead of a time.Duration, which overflows 292 years after the epoch
	return time.Unix(julianEpoch().Unix()+int64(asSeconds), 0).UTC()
}

func uuidFromString(s string) uuid.UUID {
	u, err := uuid.Parse(s)
	if err != nil {
		return uuid.New()
	}

	return u
}
