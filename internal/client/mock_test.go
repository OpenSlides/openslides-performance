package client_test

import "net/http"

const authToken42 = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOjQyfQ.C-JljH1Iy2Oe4qTSQ_1LSSDXA7j7l3d4IV3M0MzSbS8"

type serverStub struct {
	mux *http.ServeMux

	authStatus            int
	authToken, authCookie string

	backendReturnStatus int
	backendReturnBody   string
	autoupdateMessages  <-chan string
}

func newServerSub() *serverStub {
	s := serverStub{
		mux:        http.NewServeMux(),
		authToken:  authToken42,
		authCookie: "auth-cookie",
		authStatus: 200,

		backendReturnStatus: 200,
	}

	s.mux.Handle("/system/auth/login", http.HandlerFunc(s.handleAuth))
	s.mux.Handle("/system/action/handle_request", http.HandlerFunc(s.handleBackendAction))
	s.mux.Handle("/system/autoupdate", http.HandlerFunc(s.handleAutoupdate))

	return &s

}

func (s *serverStub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *serverStub) handleAuth(w http.ResponseWriter, r *http.Request) {
	if s.authStatus != 200 {
		w.WriteHeader(s.authStatus)
		return
	}

	w.Header().Add("authentication", s.authToken)
	cookie := http.Cookie{
		Name:  "refreshId",
		Value: s.authCookie,
	}
	http.SetCookie(w, &cookie)
}

func (s *serverStub) handleBackendAction(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(s.backendReturnStatus)
	w.Write([]byte(s.backendReturnBody))
}

func (s *serverStub) handleAutoupdate(w http.ResponseWriter, r *http.Request) {
	for m := range s.autoupdateMessages {
		w.Write([]byte(m))
		w.Write([]byte("\n"))
		w.(http.Flusher).Flush()
	}
}
