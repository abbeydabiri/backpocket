package users

type users struct {
	ID       uint64 `sql:"index"`
	Symbol   string `sql:"index"`
	Status   string `sql:"index"`
	Exchange string `sql:"index"`
	State    string `sql:"index"` //buy or sell
	Address  string `sql:"index"`
	Free     float64
	Locked   float64
}

// func HandleRoutes(muxrouter mux.Router) {

// 	muxrouter.HandleFunc("/signin", signinHandler).Methods("POST")
// 	muxrouter.HandleFunc("/signup", signupHandler).Methods("POST")
// }
