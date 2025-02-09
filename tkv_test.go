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
	"testing"
	"time"

	"github.com/johnknl/rtkv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	batchSize    = 5_000
	benchRecords = 100_000
)

func TestRedisTKV_CRUD(t *testing.T) {
	ctx := context.Background()

	redisClient := newGoRedisClient(0)

	t.Cleanup(func() {
		redisClient.FlushDB(ctx).Err()
	})

	store := rtkv.NewRedisTKV(rtkv.DelimUnit, t.Name(), redisClient)

	now := time.Now()
	id := []string{"a", "a"}
	data := []byte(`{"id": "a"}`)
	lastModified := time.Now()

	t.Run("Set", func(t *testing.T) {
		existed, err := store.Set(ctx, data, lastModified, id...)

		require.NoErrorf(t, err, "Set should not return an error")
		assert.Falsef(t, existed, "Entity should not exist before first insert")

		exists, err := store.Exists(ctx, id...)

		require.NoErrorf(t, err, "Exists should not return an error")
		assert.Truef(t, exists, "Entity should exist after being set")
	})

	t.Run("BulkSet", func(t *testing.T) {
		require.NoError(t, store.BulkSet(ctx, nil))

		err := store.BulkSet(ctx, []rtkv.BulkSetRecord{
			{Data: []byte(`{"id": "b"}`), ID: []string{"a", "b", "b"}, LastModified: now.Add(-time.Minute)},
			{Data: []byte(`{"id": "c"}`), ID: []string{"a", "b", "c"}, LastModified: now.Add(-2 * time.Minute)},
			{Data: []byte(`{"id": "d"}`), ID: []string{"a", "b", "d"}, LastModified: now.Add(-3 * time.Minute)},
			{Data: []byte(`{"id": "e"}`), ID: []string{"a", "b", "e"}, LastModified: now.Add(-4 * time.Hour)},
		})

		require.NoError(t, err)
	})

	t.Run("Get", func(t *testing.T) {
		foundData, err := store.Get(ctx, id...)

		require.NoErrorf(t, err, "Get should not return an error")
		assert.Equalf(t, data, foundData, "Get should return the correct data")
	})

	t.Run("Delete", func(t *testing.T) {
		err := store.Delete(ctx, id...)

		require.NoErrorf(t, err, "Delete should not return an error")

		exists, err := store.Exists(ctx, id...)

		require.NoErrorf(t, err, "Exists should not return an error")
		assert.Falsef(t, exists, "Entity should not exist after being deleted")
	})
}
