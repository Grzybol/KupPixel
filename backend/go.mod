module github.com/example/kup-piksel

go 1.21.0

toolchain go1.24.1

require (
        github.com/gin-gonic/gin v0.0.0
        github.com/go-sql-driver/mysql v1.9.3
        github.com/mattn/go-sqlite3 v1.14.22
        golang.org/x/crypto/bcrypt v0.0.0
)

require (
        filippo.io/edwards25519 v1.1.0 // indirect
)

replace github.com/gin-gonic/gin => ./internal/ginlite

replace github.com/mattn/go-sqlite3 => ./internal/sqlite3

replace golang.org/x/crypto/bcrypt => ./internal/bcrypt
