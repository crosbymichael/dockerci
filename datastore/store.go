package datastore

import (
	"errors"
	"github.com/garyburd/redigo/redis"
	"path"
)

const (
	DEFAULT_POOL_SIZE = 10 // Number of redis connections to keep in the pool
	DEFAULT_TIMEOUT   = 0  // Defaults to 0 to block forever
)

var (
	ErrKeyIsAlreadySet = errors.New("key is already set")
)

type Store struct {
	pool *redis.Pool
}

// New returns a new Store with a redis pool for the
// given address
func New(addr string) *Store {
	return &Store{
		pool: newPool(addr),
	}
}

func (s *Store) Close() error {
	return s.pool.Close()
}

func (s *Store) AtomicSaveState(repository, commit, state string) error {
	isSet, err := redis.Int(s.do("SETNX", path.Join("/dockerci", repository, "commit", commit), state))
	if err != nil {
		return err
	}
	if isSet == 0 {
		return ErrKeyIsAlreadySet
	}
	return nil
}

func (s *Store) do(cmd string, args ...interface{}) (interface{}, error) {
	conn := s.pool.Get()
	defer conn.Close()
	return conn.Do(cmd, args...)
}

func newPool(addr string) *redis.Pool {
	return redis.NewPool(func() (redis.Conn, error) {
		return redis.Dial("tcp", addr)
	}, DEFAULT_POOL_SIZE)
}
