package memory

import (
	"fmt"
	"regexp"
	"sort"
	"sync"

	"github.com/nxshock/signaller/internal/models/createroom"

	"github.com/nxshock/signaller/internal"
	"github.com/nxshock/signaller/internal/models"
	mSync "github.com/nxshock/signaller/internal/models/sync"
)

type Backend struct {
	data                 map[string]internal.User
	rooms                map[string]internal.Room
	hostname             string
	validateUsernameFunc func(string) error // TODO: create ability to redefine validation func
	mutex                sync.Mutex         // TODO: replace with RW mutex
}

type Token struct {
	Device string
}

func NewBackend(hostname string) *Backend {
	return &Backend{
		hostname:             hostname,
		validateUsernameFunc: defaultValidationUsernameFunc,
		rooms:                make(map[string]internal.Room),
		data:                 make(map[string]internal.User)}
}

func (backend *Backend) Register(username, password, device string) (user internal.User, token string, err models.ApiError) {
	backend.mutex.Lock()

	if backend.validateUsernameFunc != nil {
		err := backend.validateUsernameFunc(username)
		if err != nil {
			return nil, "", models.NewError(models.M_INVALID_USERNAME, err.Error())
		}
	}

	if _, ok := backend.data[username]; ok {
		backend.mutex.Unlock()
		return nil, "", models.NewError(models.M_USER_IN_USE, "trying to register a user ID which has been taken")
	}

	user = &User{
		name:     username,
		password: password,
		Tokens:   make(map[string]Token),
		backend:  backend}

	backend.data[username] = user

	backend.mutex.Unlock()
	return backend.Login(username, password, device)
}

func (backend *Backend) Login(username, password, device string) (user internal.User, token string, err models.ApiError) {
	backend.mutex.Lock()
	defer backend.mutex.Unlock()

	user, ok := backend.data[username]
	if !ok {
		return nil, "", models.NewError(models.M_FORBIDDEN, "wrong username")
	}

	if user.Password() != password {
		return nil, "", models.NewError(models.M_FORBIDDEN, "wrong password")
	}

	token = newToken(defaultTokenSize)

	backend.data[username].(*User).Tokens[token] = Token{Device: device}

	return user, token, nil
}

func (backend *Backend) Sync(token string, request mSync.SyncRequest) (response *mSync.SyncReply, err models.ApiError) {
	backend.mutex.Lock()
	defer backend.mutex.Unlock()

	return nil, nil // TODO: implement
}

func (backend *Backend) GetUserByToken(token string) internal.User {
	backend.mutex.Lock()
	defer backend.mutex.Unlock()

	for _, user := range backend.data {
		for userToken := range user.(*User).Tokens {
			if userToken == token {
				return user
			}
		}
	}

	return nil
}

func (backend *Backend) GetRoomByID(id string) internal.Room {
	backend.mutex.Lock()
	defer backend.mutex.Unlock()

	for roomID, room := range backend.rooms {
		if roomID == id {
			return room
		}
	}

	return nil
}

func (backend *Backend) GetUserByName(userName string) internal.User {
	backend.mutex.Lock()
	defer backend.mutex.Unlock()

	if user, exists := backend.data[userName]; exists {
		return user
	}

	return nil
}

func (backend *Backend) PublicRooms() []internal.Room {
	backend.mutex.Lock()
	defer backend.mutex.Unlock()

	var rooms []internal.Room

	for _, room := range backend.rooms {
		if room.State() == createroom.PublicChat {
			rooms = append(rooms, room)
		}
	}

	sort.Sort(BySize(rooms))

	return rooms
}

func (backend *Backend) ValidateUsernameFunc() func(string) error {
	backend.mutex.Lock()
	defer backend.mutex.Unlock()

	return backend.validateUsernameFunc
}

func defaultValidationUsernameFunc(userName string) error {
	const re = `^\w{5,}$`

	if !regexp.MustCompile(re).MatchString(userName) {
		return fmt.Errorf("username does not match %s", re)
	}

	return nil
}
