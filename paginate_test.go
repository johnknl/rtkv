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
	"errors"
	"iter"
	"testing"
	"time"

	"github.com/johnknl/rtkv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mockPageFunc(pages [][]byte) rtkv.PageFunc {
	return func(
		_ context.Context,
		_, _ *time.Time,
		offset, limit int,
	) (iter.Seq2[[]byte, error], int64, error) {
		if offset >= len(pages) {
			return nil, int64(len(pages)), nil
		}

		end := offset + limit

		if end > len(pages) {
			end = len(pages)
		}

		return func(yield func([]byte, error) bool) {
			for _, item := range pages[offset:end] {
				if !yield(item, nil) {
					return
				}
			}
		}, int64(len(pages)), nil
	}
}

func TestPaginate(t *testing.T) {
	ctx := context.Background()

	pages := [][]byte{
		[]byte("item1"), []byte("item2"), []byte("item3"),
		[]byte("item4"), []byte("item5"), []byte("item6"),
	}
	pageFn := mockPageFunc(pages)

	iterator, err := rtkv.Paginate(ctx, pageFn, nil, nil, 0, 2)

	require.NoErrorf(t, err, "Paginate should not return an error")

	var results [][]byte

	for item, err := range iterator {
		require.NoErrorf(t, err, "Iterator should not return errors")
		results = append(results, item)
	}

	assert.Equalf(t, pages, results, "Paginate should return all items in order")

	t.Run("EarlyExit", func(t *testing.T) {
		iterator, err = rtkv.Paginate(ctx, pageFn, nil, nil, 0, 6)

		require.NoErrorf(t, err, "Paginate should not return an error")

		for _, err = range iterator {
			require.NoErrorf(t, err, "Iterator should not return errors")
		}
	})
}

func TestPaginate_ErrorOnFirstPage(t *testing.T) {
	ctx := context.Background()

	pageFn := func(
		_ context.Context,
		_, _ *time.Time,
		_, _ int,
	) (iter.Seq2[[]byte, error], int64, error) {
		return nil, 0, errors.New("mock error")
	}

	_, err := rtkv.Paginate(ctx, pageFn, nil, nil, 0, 2)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetching first page failed")
}

func TestPaginate_ErrorOnNextPage(t *testing.T) {
	ctx := context.Background()

	firstPage := [][]byte{[]byte("item1"), []byte("item2")}

	pageFn := func(
		_ context.Context,
		_, _ *time.Time,
		offset, _ int,
	) (iter.Seq2[[]byte, error], int64, error) {
		if offset == 0 {
			return func(yield func([]byte, error) bool) {
				for _, page := range firstPage {
					if !yield(page, nil) {
						return
					}
				}
			}, 4, nil
		}

		return nil, 4, errors.New("mock error on next page")
	}

	iterator, err := rtkv.Paginate(ctx, pageFn, nil, nil, 0, 3)

	require.NoErrorf(t, err, "Paginate should not return an error immediately")

	var results [][]byte
	var encounteredErr error

	for item, err := range iterator {
		if err != nil {
			encounteredErr = err
			break
		}
		results = append(results, item)
	}

	// Ensure the first page was processed correctly
	assert.Equalf(t, firstPage, results, "Paginate should return items before error")

	// Ensure an error was encountered on the second page fetch
	require.Errorf(t, encounteredErr, "An error should be encountered on the second page")
	assert.Contains(t, encounteredErr.Error(), "fetching next page failed")
}
