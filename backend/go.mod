module github.com/example/kup-piksel

go 1.21

require (
	github.com/gin-gonic/gin v0.0.0
	github.com/mattn/go-sqlite3 v1.14.22
	golang.org/x/crypto/bcrypt v0.0.0
)

replace github.com/gin-gonic/gin => ./internal/ginlite

replace github.com/mattn/go-sqlite3 => ./internal/sqlite3

replace golang.org/x/crypto/bcrypt => ./internal/bcrypt
