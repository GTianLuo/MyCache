package MyCache

import (
	"errors"
	"log"
	"sync"
)

type Getter interface {
	Get(key string) ([]byte, error)
}

type Group struct {
	name      string
	mainCache *cache
	getter    Getter
	peers     PeerPicker
}

var (
	rw     sync.RWMutex
	groups = make(map[string]*Group)
)

type GetterFunc func(string) ([]byte, error)

func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

func NewGroup(name string, cacheBytes uint64, getter Getter) *Group {
	if getter == nil {
		panic("getter is not allowed nil")
	}
	rw.Lock()
	defer rw.Unlock()
	group := Group{
		name:      name,
		mainCache: &cache{maxBytes: cacheBytes},
		getter:    getter,
	}
	groups[name] = &group
	return &group
}

func GetGroup(name string) *Group {
	rw.RLock()
	defer rw.RUnlock()
	return groups[name]
}

func (g *Group) Get(key string) (ByteValue, error) {
	if key == "" {
		return ByteValue{}, errors.New("key is required")
	}
	if v, ok := g.mainCache.get(key); ok {
		log.Printf("[MyCache] %s hit int cache", key)
		return v, nil
	}
	return g.load(key)
}

func (g *Group) getLocally(key string) (ByteValue, error) {
	v, err := g.getter.Get(key)
	if err != nil {
		return ByteValue{}, err
	}
	b := ByteValue{b: bytesClone(v)}
	return g.populateCache(key, b)
}

func (g *Group) populateCache(key string, b ByteValue) (ByteValue, error) {
	err := g.mainCache.add(key, b)
	if err != nil {
		return ByteValue{}, nil
	}
	return b, nil
}

func (g *Group) RegisterPeers(picker PeerPicker) {
	if g.peers != nil {
		panic("this group had registered")
	}
	g.peers = picker
}

func (g *Group) load(key string) (ByteValue, error) {
	if g.peers != nil {
		if getter, ok := g.peers.PickPeer(key); ok {
			return g.getFormPeer(key, getter)
		}
	}
	return g.getLocally(key)
}

func (g *Group) getFormPeer(key string, getter PeerGetter) (ByteValue, error) {
	bytes, err := getter.Get(g.name, key)
	if err != nil {
		return ByteValue{}, err
	}
	return ByteValue{bytes}, nil
}
