// GNU AFFERO GENERAL PUBLIC LICENSE
// Version 3, 19 November 2007
//
// Copyright (C) 2025 John Kleijn
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.
//
// For more details, see the full AGPL-3.0 license at:
// https://www.gnu.org/licenses/agpl-3.0.html

package rtkv_test

import (
	"context"
	"fmt"
	"time"

	"github.com/johnknl/rtkv"
)

func ExampleRedisTKV() {
	ctx := context.Background()
	store := rtkv.NewRedisTKV(rtkv.DelimUnit, "example", newGoRedisClient(0))
	now := time.Now()

	// Set the value of entity "a", "a"
	existed, err := store.Set(ctx, []byte(`{"id": "a"}`), now, "a", "a")

	fmt.Println(existed, err)

	// Get the value of id "a" ([]byte(nil))
	val, err := store.Get(ctx, "a")

	fmt.Printf("%#v %v\n", val, err)

	// Get the value of id "a", "a" ({"id": "a"}, <nil>)
	val, err = store.Get(ctx, "a", "a")
	fmt.Printf("%s %v\n", val, err)

	// Bulk set some entities
	_ = store.BulkSet(ctx, []rtkv.BulkSetRecord{
		{Data: []byte(`{"id": "b"}`), ID: []string{"a", "b", "b"}, LastModified: now.Add(-time.Minute)},
		{Data: []byte(`{"id": "c"}`), ID: []string{"a", "b", "c"}, LastModified: now.Add(-2 * time.Minute)},
		{Data: []byte(`{"id": "d"}`), ID: []string{"a", "b", "d"}, LastModified: now.Add(-3 * time.Minute)},
		{Data: []byte(`{"id": "e"}`), ID: []string{"a", "b", "e"}, LastModified: now.Add(-4 * time.Hour)},
	})

	// Get max 2 entities from a range that matches 3 in a set of 5 (oldest first)
	from := now.Add(-3 * time.Minute)
	to := now.Add(-time.Minute)
	iterator, total, err := store.FetchPage(ctx, &from, &to, 0, 2)

	fmt.Println(
		"err:", err,
		"total:", total,
	)

	for data, err := range iterator {
		fmt.Println(string(data), err)
	}

	// Output:
	// false <nil>
	// []byte(nil) <nil>
	// {"id": "a"} <nil>
	// err: <nil> total: 3
	// {"id": "d"} <nil>
	// {"id": "c"} <nil>
}
