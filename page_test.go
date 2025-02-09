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
	"encoding/json"
	"testing"
	"time"

	"github.com/johnknl/rtkv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func BenchmarkRedisTKV_FetchPage(b *testing.B) {
	store := goRedisSetup(b, benchRecords)

	limit := batchSize
	from := time.Now().Add(-time.Minute)
	to := time.Now()

	runCtx, cancel := context.WithCancel(context.Background())

	defer cancel()

	runFetchPage := func(b *testing.B, fn rtkv.PageFunc) {
		b.Helper()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				it, err := rtkv.Paginate(runCtx, fn, &from, &to, 0, limit)

				require.NoError(b, err)

				i := 0

				for data, err := range it {
					require.NoError(b, err)

					i++
					_ = data
				}

				assert.Equal(b, benchRecords, i)
			}
		})
	}

	b.ResetTimer()

	b.Run("Default", func(b *testing.B) {
		runFetchPage(b, store.FetchPage)
	})

	b.Run("Consistent", func(b *testing.B) {
		runFetchPage(b, store.FetchPageConsistent)
	})
}

func TestRedisTKV_FetchPage(t *testing.T) {
	const testSetSize = 1000

	runCtx, cancel := context.WithCancel(context.Background())

	defer cancel()

	runFetchAll := func(t *testing.T, fn rtkv.PageFunc) {
		t.Helper()

		from := time.Now().Add(-time.Minute)
		to := time.Now()
		offset, limit := 0, testSetSize/10

		var v map[string]any

		it, err := rtkv.Paginate(runCtx, fn, &from, &to, offset, limit)

		require.NoErrorf(t, err, "Paginate should not return an error")

		var i int

		for b, err := range it {
			require.NoErrorf(t, err, "Iterator should not return an error")

			i++
			err = json.Unmarshal(b, &v)
			require.NoErrorf(t, err, "Unmarshal should not return an error")
		}

		assert.Equalf(t, testSetSize, i, "FetchPage should return the correct batch size")
	}

	store := goRedisSetup(t, testSetSize)

	t.Run("Default", func(t *testing.T) {
		t.Run("FetchPage all", func(t *testing.T) {
			runFetchAll(t, store.FetchPage)
		})
	})

	t.Run("Consistent", func(t *testing.T) {
		t.Run("FetchPage all", func(t *testing.T) {
			runFetchAll(t, store.FetchPageConsistent)
		})
	})
}
