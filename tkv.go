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
	"errors"
	"fmt"
	"iter"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/go-redis/redis/v8"
)

var ErrUnexpectedScriptResult = errors.New("unexpected result from lua script")

const (
	// DelimUnit is ASCII unit separator character.
	// Use this as a delimeter for keys if you need
	// a non-printable character. Safest choice.
	DelimUnit = "\x1f"

	// DelimPipe is ASCII pipe character.
	// Use this as a separator in keys if you need
	// a printable character. May be easier to debug.
	DelimPipe = "|"

	lastModifiedIdxSuffix = "lmIdx"

	// RangeScript is a lua script that will return a range of elements
	// from a sorted set. The script will return the total number of
	// elements in the range and the values of the elements.
	// The script is executed atomically, preventing range getting
	// out of sync with the keys it references.
	rangeScript = `
local key = KEYS[1] -- the sorted set key
local min = ARGV[1] -- the minimum score
local max = ARGV[2] -- the maximum score
local offset = tonumber(ARGV[3]) -- the offset relative to the first element in the score range
local count = tonumber(ARGV[4]) -- the max size of the result set

local total = redis.call("ZCOUNT", key, min, max)
if total == 0 then
  return { 0, {} }
end

local keys = redis.call("ZRANGE", key, min, max, "BYSCORE", "LIMIT", offset, count)
if #keys == 0 then
  return { 0, {} }
end

return { total, redis.call("MGET", unpack(keys)) }
`
)

type BulkSetRecord struct {
	LastModified time.Time
	ID           []string
	Data         []byte
}

// RedisTKV is a k/v store backed by Redis.
// It uses a sorted set to keep track of last
// modified time and enable range queries.
type RedisTKV struct {
	client      *redis.Client
	namespace   string
	idDelimiter string
	scriptSHA   string
	shaMx       sync.Mutex
}

// NewRedisTKV creates a new RedisTKV instance.
// The namespace is used to prefix keys in Redis.
//
// The `idDelimiter` argument is used as a namespace
// delimeter and to pack composite IDs into a single key.
//
// The `namespace` argument prevents key collisions
// for different entitiy types.
func NewRedisTKV(idDelimiter, namespace string, c *redis.Client) *RedisTKV {
	return &RedisTKV{
		client:      c,
		namespace:   namespace,
		idDelimiter: idDelimiter,
	}
}

// Get an entity by ID.
func (r *RedisTKV) Get(ctx context.Context, id ...string) ([]byte, error) {
	data, err := r.client.Get(ctx, r.namespacedKey(id...)).Bytes()

	if errors.Is(err, redis.Nil) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get entity: %w", err)
	}

	return data, nil
}

// BulkSet sets multiple entities in the store.
func (r *RedisTKV) BulkSet(ctx context.Context, records []BulkSetRecord) error {
	if len(records) == 0 {
		return nil
	}

	_, err := r.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		for i := range records {
			timestamp := records[i].LastModified.UnixNano()
			key := r.namespacedKey(records[i].ID...)

			pipe.Set(ctx, key, records[i].Data, 0)
			pipe.ZAdd(ctx, r.namespacedKey(lastModifiedIdxSuffix), &redis.Z{
				Score:  float64(timestamp),
				Member: key,
			})
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to bulk insert records: %w", err)
	}

	return nil
}

// Set an entity in the store by ID.
// If the entity already exists, it will be overwritten.
// Returns boolean true if entity already existed.
func (r *RedisTKV) Set(ctx context.Context, data []byte, lastModified time.Time, id ...string) (bool, error) {
	timestamp := lastModified.UnixNano()
	key := r.namespacedKey(id...)

	var zaddRes *redis.IntCmd

	_, err := r.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Set(ctx, key, data, 0)

		zaddRes = pipe.ZAdd(ctx, r.namespacedKey(lastModifiedIdxSuffix), &redis.Z{
			Score:  float64(timestamp),
			Member: key,
		})

		return nil
	})
	if err != nil {
		return false, fmt.Errorf("failed to set entity: %w", err)
	}

	return zaddRes.Val() == 0, nil
}

func (r *RedisTKV) Exists(ctx context.Context, id ...string) (bool, error) {
	result, err := r.client.Exists(ctx, r.namespacedKey(id...)).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check if entity exists: %w", err)
	}

	return result > 0, nil
}

func (r *RedisTKV) Delete(ctx context.Context, id ...string) error {
	_, err := r.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Del(ctx, r.namespacedKey(id...))
		pipe.ZRem(ctx, r.namespacedKey(lastModifiedIdxSuffix), id)

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to delete entity: %w", err)
	}

	return nil
}

func (r *RedisTKV) FetchPage(
	ctx context.Context,
	from, to *time.Time, //nolint:varnamelen // from and to are clear
	offset, limit int,
) (iter.Seq2[[]byte, error], int64, error) {
	var rangeMin, rangeMax string
	if from != nil {
		rangeMin = strconv.Itoa(int(from.UnixNano()))
	} else {
		rangeMin = "-inf"
	}

	if to != nil {
		rangeMax = strconv.Itoa(int(to.UnixNano()))
	} else {
		rangeMax = "+inf"
	}

	key := r.namespacedKey(lastModifiedIdxSuffix)

	total, err := r.client.ZCount(ctx, key, rangeMin, rangeMax).Result()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count: %w", err)
	}

	result, err := r.client.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min:    rangeMin,
		Max:    rangeMax,
		Offset: int64(offset),
		Count:  int64(limit),
	}).Result()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to execute zrangebyscore: %w", err)
	}

	if len(result) == 0 {
		return func(func([]byte, error) bool) {}, total, nil
	}

	mGetResult, err := r.client.MGet(ctx, result...).Result()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to execute mget: %w", err)
	}

	return func(yield func([]byte, error) bool) {
		for _, rawValue := range mGetResult {
			if !yield(s2b(rawValue.(string)), nil) {
				break
			}
		}
	}, total, nil
}

func (r *RedisTKV) FetchPageConsistent(
	ctx context.Context,
	from, to *time.Time, //nolint:varnamelen // from and to are clear
	offset, limit int,
) (iter.Seq2[[]byte, error], int64, error) {
	var rangeMin, rangeMax string
	if from != nil {
		rangeMin = strconv.Itoa(int(from.UnixNano()))
	} else {
		rangeMin = "-inf"
	}

	if to != nil {
		rangeMax = strconv.Itoa(int(to.UnixNano()))
	} else {
		rangeMax = "+inf"
	}

	keys := []string{r.namespacedKey(lastModifiedIdxSuffix)}
	args := []any{rangeMin, rangeMax, offset, limit}

	sha, err := r.getScriptSHA(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to load script: %w", err)
	}

	result, err := r.client.EvalSha(ctx, sha, keys, args...).Result()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to execute search.lua script: %w", err)
	}

	resultSlice, ok := result.([]any)

	if !ok || len(resultSlice) != 2 {
		return nil, 0, ErrUnexpectedScriptResult
	}

	total := resultSlice[0].(int64)
	rawValues := resultSlice[1].([]any)

	return func(yield func([]byte, error) bool) {
		for _, rawValue := range rawValues {
			if !yield(s2b(rawValue.(string)), nil) {
				break
			}
		}
	}, total, nil
}

func (r *RedisTKV) namespacedKey(key ...string) string {
	return r.namespace + r.idDelimiter + strings.Join(key, r.idDelimiter)
}

func (r *RedisTKV) getScriptSHA(ctx context.Context) (string, error) {
	r.shaMx.Lock()
	defer r.shaMx.Unlock()

	if r.scriptSHA != "" {
		return r.scriptSHA, nil
	}
	var err error

	r.scriptSHA, err = r.client.ScriptLoad(ctx, rangeScript).Result()
	if err != nil {
		return "", fmt.Errorf("failed to load lua range script: %w", err)
	}

	return r.scriptSHA, nil
}

func s2b(s string) (b []byte) {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}
