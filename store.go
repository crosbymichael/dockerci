package dockerci

import (
	"errors"
	"fmt"
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
func New(addr, password string) *Store {
	return &Store{
		pool: newPool(addr, password),
	}
}

func (s *Store) Close() error {
	return s.pool.Close()
}

func (s *Store) AtomicSaveState(commit, state string) error {
	isSet, err := redis.Int(s.do("SETNX", stateKey(commit), state))
	if err != nil {
		return err
	}
	if isSet == 0 {
		return ErrKeyIsAlreadySet
	}
	return nil
}

func (s *Store) SaveBuildResult(commit string, data map[string]string) error {
	// set the top level state field to done now that the build is complete
	conn := s.pool.Get()
	defer conn.Close()

	if err := conn.Send("MULTI"); err != nil {
		return err
	}
	if err := conn.Send("SET", stateKey(commit), "complete"); err != nil {
		return err
	}
	args := []interface{}{
		resultKey(commit),
	}
	for k, v := range data {
		args = append(args, k, v)
	}
	if err := conn.Send("HMSET", args...); err != nil {
		return err
	}
	if _, err := conn.Do("EXEC"); err != nil {
		return err
	}
	return nil
}

func (s *Store) IncrementRequest(action string) error {
	if _, err := s.do("INCR", path.Join("/dockerci", "stats", action, "count")); err != nil {
		return err
	}
	return nil
}

func (s *Store) SaveMessageDuration(queue string, secounds float64) error {
	if _, err := s.do("RPUSH", path.Join("/dockerci", "message-duration", queue), secounds); err != nil {
		return err
	}
	return nil
}

// SaveCommitForPullRequest saves the commit to the pr number in a set
func (s *Store) SaveCommitForPullRequest(number int, commit string) error {
	if _, err := s.do("SADD", prKey(number), commit); err != nil {
		return err
	}
	return nil
}

func (s *Store) do(cmd string, args ...interface{}) (interface{}, error) {
	conn := s.pool.Get()
	defer conn.Close()
	return conn.Do(cmd, args...)
}

func newPool(addr, password string) *redis.Pool {
	return redis.NewPool(func() (redis.Conn, error) {
		c, err := redis.Dial("tcp", addr)
		if err != nil {
			return nil, err
		}
		if password != "" {
			if _, err := c.Do("AUTH", password); err != nil {
				return nil, err
			}
		}
		return c, nil
	}, DEFAULT_POOL_SIZE)
}

func stateKey(commit string) string {
	return path.Join("/dockerci", "commit", commit, "state")
}

func prKey(number int) string {
	return path.Join("/dockerci", "pullrequests", fmt.Sprint(number))
}

func resultKey(commit string) string {
	return path.Join("/dockerci", "commit", commit, "result")
}
