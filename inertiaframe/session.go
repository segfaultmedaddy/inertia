package inertiaframe

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.inout.gg/foundations/http/httpcookie"

	"go.inout.gg/inertia"
)

type sessCtx struct{}

var kSessCtx = sessCtx{} //nolint:gochecknoglobals

const (
	SessionCookieName = "_inertiaframe"
	SessionPath       = "/"
)

//nolint:gochecknoglobals
var bufPool = sync.Pool{New: func() any { return bytes.NewBuffer(nil) }}

//nolint:gochecknoinits
func init() {
	gob.Register(&session{}) //nolint:exhaustruct
	gob.Register([]inertia.ValidationError(nil))
}

// Session stores temporary flash data for the inertiaframe package.
// It manages validation errors and the last visited path for redirect-back functionality.
// Session data is stored in a cookie and automatically cleared after being read.
type session struct {
	ErrorBag_         string                    //nolint:revive
	Path_             string                    //nolint:revive
	ValidationErrors_ []inertia.ValidationError //nolint:revive
}

// sessionFromRequest retrieves a session from the request. If the session
// does not exist, a new session is created.
func sessionFromRequest(r *http.Request) (*session, error) {
	sess, ok := r.Context().Value(kSessCtx).(*session)
	if ok && sess != nil {
		return sess, nil
	}

	val := httpcookie.Get(r, SessionCookieName)
	if val == "" {
		//nolint:exhaustruct
		return &session{}, nil
	}

	b, err := base64.RawURLEncoding.DecodeString(val)
	if err != nil {
		return nil, fmt.Errorf("inertiaframe: failed to decode session cookie: %w", err)
	}

	sess = &session{} //nolint:exhaustruct
	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(sess); err != nil {
		return nil, fmt.Errorf("inertiaframe: failed to decode session: %w", err)
	}

	// Save session for future requests.
	*r = *r.WithContext(context.WithValue(r.Context(), kSessCtx, sess))

	return sess, nil
}

// ValidationErrors returns validation errors from the previous request.
// Errors are automatically cleared after being read (flash behavior).
func (s *session) ValidationErrors() []inertia.ValidationError {
	ret := s.ValidationErrors_
	s.ValidationErrors_ = nil

	return ret
}

// ErrorBag returns the error bag name from the previous request that produced errors.
// Automatically cleared after being read.
func (s *session) ErrorBag() string {
	ret := s.ErrorBag_
	s.ErrorBag_ = ""

	return ret
}

// Referer returns the last visited path stored in the session.
// Used by RedirectBack to navigate to the previous page.
func (s *session) Referer() string { return s.Path_ }

// Clear deletes the session cookie from the client.
func (s *session) Clear(w http.ResponseWriter, r *http.Request) {
	httpcookie.Delete(w, r, SessionCookieName)
}

// Save persists the session to a cookie sent to the client.
func (s *session) Save(w http.ResponseWriter) error {
	buf := bufPool.Get().(*bytes.Buffer) //nolint:forcetypeassert

	defer func() {
		bufPool.Put(buf)
		buf.Reset()
	}()

	err := gob.NewEncoder(buf).Encode(s)
	if err != nil {
		return fmt.Errorf("inertiaframe: failed to encode session: %w", err)
	}

	//nolint:exhaustruct
	cookie := &http.Cookie{
		Name:     SessionCookieName,
		Value:    base64.RawURLEncoding.EncodeToString(buf.Bytes()),
		Path:     SessionPath,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now(),
	}

	http.SetCookie(w, cookie)

	return nil
}
