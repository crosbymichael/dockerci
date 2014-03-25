package datastore

import (
	"github.com/garyburd/redigo/redis"
	"path"
)

const (
	DEFAULT_POOL_SIZE = 10 // Number of redis connections to keep in the pool
	DEFAULT_TIMEOUT   = 0  // Defaults to 0 to block forever
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

// SavePullRequest will save the raw json as a blob for a given repository
func (s *Store) SavePullRequest(repository, uuid string, blob []byte) error {
	key := path.Join("/dockerci", repository, "pullrequest", uuid, "blob")
	_, err := s.do("SET", key, blob)
	return err
}

// FetchPullRequest will return the blog data for a given pull request uuid in a repository
func (s *Store) FetchPullRequest(repository, uuid string) ([]byte, error) {
	key := path.Join("/dockerci", repository, "pullrequest", uuid, "blob")
	return redis.Bytes(s.do("GET", key))
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
