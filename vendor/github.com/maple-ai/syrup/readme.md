Open source go package to enhance gorilla/mux

## Features
- Middleware
- Convenience .get, .post, ...
- Sub-routing, sub-middleware declarations

## Demo Usage

```go
r := syrup.New(mux.NewRouter())

// Authentication
func(api syrup.Router) {
	api.Post("/password", login)
	api.Post("/register", signup)
}(r.Group("/auth"))

// Limited to requests with sessions
r.Use(secureMiddleware)

// Fetch user
r.Get("/user", getUser)
// Update user details
r.Post("/user", updateUser)
// Update user password
r.Post("/user/password", updateUserPassword)

func secureMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if true {
			next.ServeHTTP(w, r)
			return
		}

		w.WriteHeader(http.StatusForbidden)
	})
}
```
