package gobbyclient

type Gobbyclient interface {
	Put(key, value string) error
	Get(key string) (string, error)
	Acquire(key string) (string, error)
	Release(key, lockstamp string) error
	Watch(key string) error
}
