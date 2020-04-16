package storage

import (
	"github.com/danieldin95/lightstar/libstar"
	"github.com/danieldin95/openlan-go/controller/schema"
	"sync"
)

type Users struct {
	Lock  sync.RWMutex
	File  string
	Users map[string]*schema.User `json:"user"`
}

func (u *Users) Save() error {
	u.Lock.RLock()
	defer u.Lock.RUnlock()

	if err := libstar.JSON.MarshalSave(&u.Users, u.File, true); err != nil {
		return err
	}
	return nil
}

func (u *Users) Load(file string) error {
	u.Lock.Lock()
	defer u.Lock.Unlock()

	u.File = file
	if err := libstar.JSON.UnmarshalLoad(&u.Users, file); err != nil {
		return err
	}
	for name, value := range u.Users {
		if value == nil {
			continue
		}
		if value.Name == "" {
			value.Name = name
		}
	}
	return nil
}

func (u *Users) Get(name string) (schema.User, bool) {
	u.Lock.RLock()
	defer u.Lock.RUnlock()

	user, ok := u.Users[name]
	if user == nil {
		return schema.User{}, false
	}
	return *user, ok
}

func (u *Users) List() <-chan *schema.User {
	c := make(chan *schema.User, 128)
	go func() {
		u.Lock.RLock()
		defer u.Lock.RUnlock()

		for _, d := range u.Users {
			c <- d
		}
		c <- nil // Finish channel by nil.
	}()
	return c
}
