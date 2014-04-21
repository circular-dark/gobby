package chubbyclient

type Chubbyclient interface {
    Put(key, value string) error
    Get(key string) (string, error)
    Aquire(key string) error
    Release(key string) error
}
