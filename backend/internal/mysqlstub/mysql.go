package mysql

import (
	"database/sql"
	"database/sql/driver"
	"errors"
)

func init() {
	sql.Register("mysql", stubDriver{})
}

type stubDriver struct{}

func (stubDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("mysql driver not available in test environment")
}
