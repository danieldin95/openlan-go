package store

import (
	"github.com/danieldin95/openlan-go/src/libol"
	"github.com/danieldin95/openlan-go/src/models"
	"sync"
	"time"
)

type _user struct {
	Lock    sync.RWMutex
	File    string
	Users   *libol.SafeStrMap
	LdapCfg *libol.LDAPConfig
	LdapSvc *libol.LDAPService
}

func (w *_user) Save() error {
	if w.File == "" {
		return nil
	}
	fp, err := libol.OpenTrunk(w.File)
	if err != nil {
		return err
	}
	for obj := range w.List() {
		if obj == nil {
			break
		}
		if obj.Role == "ldap" {
			continue
		}
		line := obj.Id() + ":" + obj.Password + ":" + obj.Role
		_, _ = fp.WriteString(line + "\n")
	}
	return nil
}

func (w *_user) SetFile(value string) {
	w.File = value
}

func (w *_user) Init(size int) {
	w.Users = libol.NewSafeStrMap(size)
}

func (w *_user) Add(user *models.User) {
	libol.Debug("_user.Add %v", user)
	key := user.Id()
	older := w.Get(key)
	if older == nil {
		_ = w.Users.Set(key, user)
	} else { // Update pass and role.
		older.Role = user.Role
		older.Password = user.Password
		older.Alias = user.Alias
		older.UpdateAt = user.UpdateAt
	}
}

func (w *_user) Del(key string) {
	libol.Debug("_user.Add %s", key)
	w.Users.Del(key)
}

func (w *_user) Get(key string) *models.User {
	if v := w.Users.Get(key); v != nil {
		return v.(*models.User)
	}
	return nil
}

func (w *_user) List() <-chan *models.User {
	c := make(chan *models.User, 128)

	go func() {
		w.Users.Iter(func(k string, v interface{}) {
			c <- v.(*models.User)
		})
		c <- nil //Finish channel by nil.
	}()

	return c
}

func (w *_user) CheckLdap(obj *models.User) *models.User {
	svc := w.GetLdap()
	if svc == nil {
		return nil
	}
	u := w.Get(obj.Id())
	libol.Debug("CheckLdap %s", u)
	if u != nil && u.Role != "ldap" {
		return nil
	}
	if ok, err := svc.Login(obj.Id(), obj.Password); !ok {
		libol.Warn("CheckLdap %s", err)
		return nil
	}
	user := &models.User{
		Name:     obj.Id(),
		Password: obj.Password,
		Role:     "ldap",
		Alias:    obj.Alias,
	}
	user.Update()
	w.Add(user)
	return user
}

func (w *_user) Timeout(user *models.User) bool {
	if user.Role == "ldap" {
		return time.Now().Unix()-user.UpdateAt > w.LdapCfg.Timeout
	}
	return true
}

func (w *_user) Check(obj *models.User) *models.User {
	if u := w.Get(obj.Id()); u != nil {
		if u.Role == "ldap" {
			// check it by ldap.
		} else {
			if u.Password == obj.Password {
				return u
			}
		}
	}
	if u := w.CheckLdap(obj); u != nil {
		return u
	}
	return nil
}

func (w *_user) GetLdap() *libol.LDAPService {
	w.Lock.Lock()
	defer w.Lock.Unlock()
	if w.LdapCfg == nil {
		return nil
	}
	if w.LdapSvc == nil || w.LdapSvc.Conn.IsClosing() {
		if l, err := libol.NewLDAPService(*w.LdapCfg); err != nil {
			libol.Warn("_user.GetLdap %s", err)
			w.LdapSvc = nil
		} else {
			w.LdapSvc = l
		}
	}
	return w.LdapSvc
}

func (w *_user) SetLdap(cfg *libol.LDAPConfig) {
	w.Lock.Lock()
	defer w.Lock.Unlock()
	if w.LdapCfg != cfg {
		w.LdapCfg = cfg
	}
	if l, err := libol.NewLDAPService(*cfg); err != nil {
		libol.Warn("_user.SetLdap %s", err)
	} else {
		libol.Info("_user.SetLdap %s", w.LdapCfg.Server)
		w.LdapSvc = l
	}
}

var User = _user{
	Users: libol.NewSafeStrMap(1024),
}
