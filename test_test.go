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
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/johnknl/rtkv"
)

func goRedisSetup(tb testing.TB, records int) *rtkv.RedisTKV {
	tb.Helper()

	client := newGoRedisClient(0)
	store := newRTKV(tb, client)

	client.FlushDB(context.Background())

	insertTestData(store, records)

	return store
}

func newGoRedisClient(db int) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   db,
	})
}

func newRTKV(tb testing.TB, c *redis.Client) *rtkv.RedisTKV {
	tb.Helper()
	return rtkv.NewRedisTKV(rtkv.DelimUnit, tb.Name(), c)
}

func insertTestData(store *rtkv.RedisTKV, totalRecords int) {
	ctx := context.Background()
	batchSize := totalRecords / 10
	records := make([]rtkv.BulkSetRecord, batchSize)

	for i := 0; i < totalRecords; i += batchSize {
		for j := range batchSize {
			index := i + j
			randomLength := rand.IntN(1000)

			records[j] = rtkv.BulkSetRecord{
				ID:           []string{"entity", strconv.Itoa(index)},
				Data:         []byte(fmt.Sprintf(`{"name":"entity_%d","value":"%s"}`, index, strings.Repeat("x", randomLength))),
				LastModified: time.Now(),
			}
		}

		err := store.BulkSet(ctx, records)
		if err != nil {
			panic("Bulk insert failed: " + err.Error())
		}
	}
}
