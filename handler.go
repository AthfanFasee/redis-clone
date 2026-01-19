package main

import (
	"maps"
	"sync"
)

const (
	// Value type names
	TYPE_STRING  = "string"
	TYPE_ERROR   = "error"
	TYPE_INTEGER = "integer"
	TYPE_BULK    = "bulk"
	TYPE_ARRAY   = "array"
	TYPE_NULL    = "null"
)

var Handlers = map[string]func([]Value) Value{
	"PING":    ping,
	"EXISTS":  exists,
	"SET":     set,
	"GET":     get,
	"DEL":     del,
	"HSET":    hset,
	"HGET":    hget,
	"HGETALL": hgetall,
	"HDEL":    hdel,
}

// ping responds with PONG or echoes back the provided argument.
func ping(args []Value) Value {
	if len(args) == 0 {
		return Value{typ: TYPE_STRING, str: "PONG"}
	}
	return Value{typ: TYPE_STRING, str: args[0].bulk}
}

// RedisStore handles simple key-value storage with thread-safe operations.
type RedisStore struct {
	mu   sync.RWMutex
	data map[string]string
}

func NewRedisStore() *RedisStore {
	return &RedisStore{
		data: make(map[string]string),
	}
}

// Set stores a key-value pair in the store.
func (s *RedisStore) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
}

// Get retrieves a value by key, returning the value and whether it exists.
func (s *RedisStore) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.data[key]
	return value, ok
}

// Delete removes a key from the store, returning whether it existed.
func (s *RedisStore) Delete(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, existed := s.data[key]
	delete(s.data, key)
	return existed
}

// Exists checks if a key exists in the store.
func (s *RedisStore) Exists(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.data[key]
	return exists
}

// HashStore handles hash (nested map) storage with thread-safe operations.
type HashStore struct {
	mu   sync.RWMutex
	data map[string]map[string]string
}

func NewHashStore() *HashStore {
	return &HashStore{
		data: make(map[string]map[string]string),
	}
}

// Set stores a field-value pair in the specified hash.
func (h *HashStore) Set(hash, key, value string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.data[hash]; !ok {
		h.data[hash] = make(map[string]string)
	}
	h.data[hash][key] = value
}

// Get retrieves a field value from a hash, returning the value and whether it exists.
func (h *HashStore) Get(hash, key string) (string, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	hashMap, ok := h.data[hash]
	if !ok {
		return "", false
	}

	value, ok := hashMap[key]
	return value, ok
}

// GetAll retrieves all field-value pairs from a hash as a copy.
func (h *HashStore) GetAll(hash string) (map[string]string, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	hashMap, ok := h.data[hash]
	if !ok {
		return nil, false
	}

	// Return a copy to prevent external modification
	result := make(map[string]string, len(hashMap))
	maps.Copy(result, hashMap)

	return result, true
}

// Delete removes a field from a hash, cleaning up empty hashes.
// Returns whether the field existed.
func (h *HashStore) Delete(hash, key string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	hashMap, ok := h.data[hash]
	if !ok {
		return false
	}

	_, existed := hashMap[key]
	delete(hashMap, key)

	// Clean up empty hash
	if len(hashMap) == 0 {
		delete(h.data, hash)
	}

	return existed
}

var (
	store     = NewRedisStore()
	hashStore = NewHashStore()
)

// set handles the SET command, storing a key-value pair.
func set(args []Value) Value {
	if len(args) != 2 {
		return Value{typ: TYPE_ERROR, str: "ERR wrong number of arguments for 'set' command"}
	}

	key := args[0].bulk
	value := args[1].bulk

	store.Set(key, value)

	return Value{typ: TYPE_STRING, str: "OK"}
}

// get handles the GET command, retrieving a value by key.
func get(args []Value) Value {
	if len(args) != 1 {
		return Value{typ: TYPE_ERROR, str: "ERR wrong number of arguments for 'get' command"}
	}

	key := args[0].bulk
	value, ok := store.Get(key)

	if !ok {
		return Value{typ: TYPE_NULL}
	}

	return Value{typ: TYPE_BULK, bulk: value}
}

// del handles the DEL command, removing one or more keys.
// Returns the number of keys that were removed.
func del(args []Value) Value {
	if len(args) == 0 {
		return Value{typ: TYPE_ERROR, str: "ERR wrong number of arguments for 'del' command"}
	}

	count := 0
	for _, arg := range args {
		key := arg.bulk
		if store.Delete(key) {
			count++
		}
	}

	return Value{typ: TYPE_INTEGER, num: count}
}

// exists handles the EXISTS command, checking if one or more keys exist.
// Returns the number of keys that exist.
func exists(args []Value) Value {
	if len(args) == 0 {
		return Value{typ: TYPE_ERROR, str: "ERR wrong number of arguments for 'exists' command"}
	}

	count := 0
	for _, arg := range args {
		key := arg.bulk
		if store.Exists(key) {
			count++
		}
	}

	return Value{typ: TYPE_INTEGER, num: count}
}

// hset handles the HSET command, storing a field-value pair in a hash.
func hset(args []Value) Value {
	if len(args) != 3 {
		return Value{typ: TYPE_ERROR, str: "ERR wrong number of arguments for 'hset' command"}
	}

	hash := args[0].bulk
	key := args[1].bulk
	value := args[2].bulk

	hashStore.Set(hash, key, value)

	return Value{typ: TYPE_STRING, str: "OK"}
}

// hget handles the HGET command, retrieving a field value from a hash.
func hget(args []Value) Value {
	if len(args) != 2 {
		return Value{typ: TYPE_ERROR, str: "ERR wrong number of arguments for 'hget' command"}
	}

	hash := args[0].bulk
	key := args[1].bulk

	value, ok := hashStore.Get(hash, key)

	if !ok {
		return Value{typ: TYPE_NULL}
	}

	return Value{typ: TYPE_BULK, bulk: value}
}

// hgetall handles the HGETALL command, retrieving all field-value pairs from a hash.
// Returns a flat array: [field1, value1, field2, value2, ...].
func hgetall(args []Value) Value {
	if len(args) != 1 {
		return Value{typ: TYPE_ERROR, str: "ERR wrong number of arguments for 'hgetall' command"}
	}

	hash := args[0].bulk

	hashMap, ok := hashStore.GetAll(hash)

	if !ok {
		// Return empty array for non-existent hash
		return Value{typ: TYPE_ARRAY, array: []Value{}}
	}

	result := make([]Value, 0, len(hashMap)*2)
	for k, v := range hashMap {
		result = append(result, Value{typ: TYPE_BULK, bulk: k})
		result = append(result, Value{typ: TYPE_BULK, bulk: v})
	}

	return Value{typ: TYPE_ARRAY, array: result}
}

// hdel handles the HDEL command, removing one or more fields from a hash.
// Returns the number of fields that were removed.
func hdel(args []Value) Value {
	if len(args) < 2 {
		return Value{typ: TYPE_ERROR, str: "ERR wrong number of arguments for 'hdel' command"}
	}

	hash := args[0].bulk
	count := 0

	for i := 1; i < len(args); i++ {
		field := args[i].bulk
		if hashStore.Delete(hash, field) {
			count++
		}
	}

	return Value{typ: TYPE_INTEGER, num: count}
}
