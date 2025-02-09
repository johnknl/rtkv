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

package rtkv

import (
	"context"
	"fmt"
	"iter"
	"time"
)

type PageFunc func(
	ctx context.Context,
	from, to *time.Time, //nolint:varnamelen // from and to are clear
	offset, limit int,
) (iter.Seq2[[]byte, error], int64, error)

func Paginate(
	ctx context.Context,
	pageFn PageFunc,
	from, to *time.Time, //nolint:varnamelen // from and to are clear
	offset, limit int,
) (iter.Seq2[[]byte, error], error) {
	it, total, err := pageFn(ctx, from, to, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("fetching first page failed: %w", err)
	}

	if int(total) <= limit {
		return it, nil
	}

	var b []byte

	return func(yield func([]byte, error) bool) {
		for {
			for b, err = range it {
				if !yield(b, err) {
					return
				}
			}

			offset += limit
			if offset >= int(total) {
				return
			}

			it, total, err = pageFn(ctx, from, to, offset, limit)
			if err != nil {
				_ = yield(nil, fmt.Errorf("fetching next page failed: %w", err))
				return
			}
		}
	}, nil
}
