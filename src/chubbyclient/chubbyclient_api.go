package chubbyclient

type Chubbyclient interface {
	Put(key, value string) error
	Get(key string) (string, error)
	Acquire(key string) (string, error)
	Release(key, lockstamp string) error
}
