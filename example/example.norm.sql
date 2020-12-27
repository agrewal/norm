-- !norm
-- Your files must start with the above line

-- !file store.go
-- Filename to write the generated output in

-- !package example
-- package name of the generated code

-- You can import packages by putting in a command like so !import "time"

-- !read GetUserListNoModel
-- !output ID int
-- !output Email string
-- !doc Retrieves all emails from the users table
SELECT id, email
FROM user
ORDER BY email ASC

-- !read GetUserEmailsNoModel
-- !output Email string
-- !doc Retrieves all emails from the users table
SELECT email
FROM user
ORDER BY email ASC

-- !read GetUserListWithModel
-- !output ID int
-- !output Email string
-- !model User
-- !doc Retrieves all emails from the users table
SELECT id, email
FROM user
ORDER BY email ASC

-- !exec AddUser
-- !input email string
-- !doc Add a user to the DB
INSERT into user(email)
VALUES ($1)

-- !exec DeleteAllUsers
-- !doc Deletes all users from the DB
DELETE FROM user

-- !read_one FindUser
-- !input email string
-- !output ID int
-- !output Email string
-- !doc Finds user by email
SELECT id, email
FROM USER
WHERE email = $1

-- !read_one FindUserEmail
-- !input email string
-- !output email string
-- !doc Finds user by email
SELECT email
FROM USER
WHERE email = $1


-- !exec CreateUserTable
-- !doc Creates the user table
CREATE TABLE user (
	id integer primary key autoincrement,
	email text
)
