// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports the tests of hayro-syntax src/object/date.rs.

package syntax

import "testing"

func TestDateTime(t *testing.T) {
	dt := func(year uint16, month, day, hour, minute, second uint8, utcHour int8, utcMinute uint8) DateTime {
		return DateTime{year, month, day, hour, minute, second, utcHour, utcMinute}
	}
	tests := []struct {
		data string
		want DateTime
	}{
		{"D:2023", dt(2023, 1, 1, 0, 0, 0, 0, 0)},
		{"D:202312", dt(2023, 12, 1, 0, 0, 0, 0, 0)},
		{"D:20231225", dt(2023, 12, 25, 0, 0, 0, 0, 0)},
		{"D:2023122514", dt(2023, 12, 25, 14, 0, 0, 0, 0)},
		{"D:202312251430", dt(2023, 12, 25, 14, 30, 0, 0, 0)},
		{"D:20231225143015", dt(2023, 12, 25, 14, 30, 15, 0, 0)},
		{"D:199812231952-08'00", dt(1998, 12, 23, 19, 52, 0, -8, 0)},
		{"D:20230701120000+05'30", dt(2023, 7, 1, 12, 0, 0, 5, 30)},
		{"D:20230701120000Z", dt(2023, 7, 1, 12, 0, 0, 0, 0)},
		{"D:20230701120000Z00'00", dt(2023, 7, 1, 12, 0, 0, 0, 0)},
		{"D:20230701120000-03'15", dt(2023, 7, 1, 12, 0, 0, -3, 15)},
		{"D:20000229010203+01'00", dt(2000, 2, 29, 1, 2, 3, 1, 0)},
		{"D:99991231235959+14'00", dt(9999, 12, 31, 23, 59, 59, 14, 0)},
		{"D:00000101000000+00'00", dt(0, 1, 1, 0, 0, 0, 0, 0)},
		{"D:202307011200+02", dt(2023, 7, 1, 12, 0, 0, 2, 0)},
		{"D:202307011200-00'45", dt(2023, 7, 1, 12, 0, 0, 0, 45)},
	}
	for _, test := range tests {
		got, ok := DateTimeFromBytes([]byte(test.data))
		if !ok || got != test.want {
			t.Errorf("%q: got %+v, %v; want %+v", test.data, got, ok, test.want)
		}
	}
}
