package two_fa_recovery

import (
	"errors"
	"fmt"
	errors2 "github.com/pkg/errors"
	"math/rand"
	"sync"
	"time"
)

type TwoFaAuthType string

const (
	TwoFaAuthTypeGoogle TwoFaAuthType = "google"
	TwoFaAuthTypeSms    TwoFaAuthType = "sms"
)

func (t TwoFaAuthType) String() string {
	return string(t)
}

var codeExpirationTime = 30 * time.Minute

type TmpCode struct {
	code         string
	nextSendTime time.Time
	timer        *time.Timer
}

type TwoFactorRecoveryService struct {
	lock        *sync.RWMutex
	codes       map[TwoFaAuthType]map[uint64]*TmpCode
	secondCodes map[TwoFaAuthType]map[uint64]*TmpCode
}

var service TwoFactorRecoveryService

func Init() {
	service = TwoFactorRecoveryService{
		lock: &sync.RWMutex{},
		codes: map[TwoFaAuthType]map[uint64]*TmpCode{
			TwoFaAuthTypeGoogle: {},
			TwoFaAuthTypeSms:    {},
		},
		secondCodes: map[TwoFaAuthType]map[uint64]*TmpCode{
			TwoFaAuthTypeGoogle: {},
			TwoFaAuthTypeSms:    {},
		},
	}
}

func CreateNewCode(userId uint64, authType TwoFaAuthType) (string, error) {
	service.lock.Lock()
	defer service.lock.Unlock()
	code := generateNewCode()

	current := service.codes[authType][userId]
	if current != nil {
		if current.nextSendTime.After(time.Now()) {
			diff := current.nextSendTime.Unix() - time.Now().Unix()
			return "", errors2.Errorf("please wait %d seconds to send next email", diff)
		} else {
			current.timer.Stop()
		}
	}

	service.codes[authType][userId] = &TmpCode{
		code:         code,
		nextSendTime: time.Now().Add(2 * time.Minute),
		timer: time.AfterFunc(codeExpirationTime, func() {
			service.lock.Lock()
			delete(service.codes[authType], userId)

			fmt.Println(service.codes)
			service.lock.Unlock()
		}),
	}

	return code, nil
}

func CreateNewSecondCode(userId uint64, authType TwoFaAuthType) (string, error) {
	service.lock.Lock()
	defer service.lock.Unlock()
	code := generateNewCode()

	current := service.secondCodes[authType][userId]
	if current != nil {
		if current.nextSendTime.After(time.Now()) {
			diff := current.nextSendTime.Unix() - time.Now().Unix()
			return "", errors2.Errorf("please wait %d seconds to send next email", diff)
		} else {
			current.timer.Stop()
		}
	}

	service.secondCodes[authType][userId] = &TmpCode{
		code:         code,
		nextSendTime: time.Now().Add(2 * time.Minute),
		timer: time.AfterFunc(codeExpirationTime, func() {
			service.lock.Lock()
			delete(service.codes[authType], userId)

			fmt.Println(service.codes)
			service.lock.Unlock()
		}),
	}

	return code, nil
}

func generateNewCode() string {
	var letters = []rune("0123456789")

	rand.Seed(time.Now().UnixNano())
	b := make([]rune, 6)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}

	return string(b)
}

func CompareCodes(userId uint64, code string, authType TwoFaAuthType) (bool, error) {
	service.lock.Lock()
	defer service.lock.Unlock()

	tmpCode, ok := service.codes[authType][userId]
	if !ok {
		return false, errors.New("code not found")
	}

	if tmpCode.code != code {
		return false, errors.New("code is wrong")
	}

	tmpCode.timer.Stop()

	return true, nil
}

func CompareSecondCodes(userId uint64, code string, authType TwoFaAuthType) (bool, error) {
	service.lock.Lock()
	defer service.lock.Unlock()

	tmpCode, ok := service.codes[authType][userId]
	if !ok {
		return false, errors.New("code not found")
	}

	if tmpCode.code != code {
		return false, errors.New("code is wrong")
	}

	tmpCode.timer.Stop()

	return true, nil
}
