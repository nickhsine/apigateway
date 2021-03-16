package token

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"firebase.google.com/go/v4/auth"

	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
}

type FirebaseToken struct {
	tokenString    *string
	tokenState     firebaseTokenState
	firebaseClient *auth.Client
}

type firebaseTokenState struct {
	sync.Mutex
	state *string
}

func (ftt *firebaseTokenState) setState(state string) {
	ftt.state = &state
}

func (ft *FirebaseToken) GetTokenString() (string, error) {
	if ft.tokenString == nil {
		return "", errors.New("token is nil")
	}
	return *ft.tokenString, nil
}

func (ft *FirebaseToken) ExecuteTokenStateUpdate() error {
	log.Debugf("ExecuteTokenStateUpdate...(token:%s)", *ft.tokenString)
	if ft.tokenString == nil {
		return errors.New("token is nil")
	}
	ft.tokenState.Lock()
	go func() {
		defer ft.tokenState.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		log.Debugf("VerifyIDTokenAndCheckRevoked...(token:%s)", *ft.tokenString)
		defer cancel()
		_, err := ft.firebaseClient.VerifyIDTokenAndCheckRevoked(ctx, *ft.tokenString)
		log.Debugf("VerifyIDTokenAndCheckRevoked Result...(token:%s)(err:%v)", *ft.tokenString, err)
		if err != nil {
			ft.tokenState.setState(err.Error())
			return
		}
		ft.tokenState.setState(OK)
	}()
	return nil
}

// GetTokenState will automatically update state if cached state is nil
func (ft *FirebaseToken) GetTokenState() string {
	if ft.tokenState.state == nil {
		ft.ExecuteTokenStateUpdate()
	}

	ft.tokenState.Lock()
	defer ft.tokenState.Unlock()
	return *ft.tokenState.state
}

// NewFirebaseToken creates a token and excute the token state update procedure
func NewFirebaseToken(authHeader *string, client *auth.Client) (Token, error) {
	if client == nil {
		return nil, errors.New("client cannot be nil")
	}
	const BearerSchema = "Bearer "
	var state, tokenString *string
	log.Debugf("NewFirebaseToken...(authHeader:%s)", *authHeader)
	if authHeader == nil || *authHeader == "" {
		s := "authorization header is not provided"
		state = &s
		log.Debugf("state is:%s)", state)
	} else if !strings.HasPrefix(*authHeader, BearerSchema) {
		s := "Not a Bearer token"
		state = &s
		log.Debugf("state is:%s)", state)
	} else {
		s := (*authHeader)[len(BearerSchema):]
		log.Debugf("trimming header :%s)", s)
		tokenString = &s
	}
	log.Debugf("final tokenString...(tokenString:%s)", tokenString)
	firebaseToken := &FirebaseToken{
		firebaseClient: client,
		tokenString:    tokenString,
		tokenState: firebaseTokenState{
			state: state,
		},
	}
	firebaseToken.ExecuteTokenStateUpdate()
	return firebaseToken, nil
}
