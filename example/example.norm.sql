-- !norm
-- Your files must start with the above line

-- !file store.go
-- Filename to write the generated output in


-- !package example
-- package name of the generated code


-- !driver_lib github.com/lib/pq
-- Database driver library

-- !driver_name postgres
-- Database driver name

-- You can import packages by putting in a command like so !import "github.com/agrewal/norm/example"

-- !read GetUserListNoModel
-- !input limit int
-- !input offset int
-- !output ID int
-- !output Email *string
-- !doc Retrieves all emails from the users table
SELECT id, email
FROM users
LIMIT $1
OFFSET $2

-- !read GetUserListWithModel
-- !input limit int
-- !input offset int
-- !output ID int
-- !output Email *string
-- !model User
-- !doc Retrieves all emails from the users table
SELECT id, email
FROM users
LIMIT $1
OFFSET $2
