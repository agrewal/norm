package example

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func TestMain(m *testing.M) {
	// Setup
	f, err := ioutil.TempFile("", "norm_test_*.db")
	if err != nil {
		panic("Could not create tmp file")
	}
	defer os.Remove(f.Name())
	db, err = sql.Open("sqlite3", fmt.Sprintf("file:%s", f.Name()))
	if err != nil {
		panic("Could not open database")
	}
	defer db.Close()
	err = CreateUserTable(db)
	if err != nil {
		panic("Could not create user table")
	}

	code := m.Run()

	os.Exit(code)
}

func deleteAllUsers() {
	err := DeleteAllUsers(db)
	if err != nil {
		panic(err)
	}
}

func TestReadOneMultiOutput(t *testing.T) {
	email := "test@dummyemail.com"
	err := AddUser(db, "test@dummyemail.com")
	if err != nil {
		panic(err)
	}
	defer deleteAllUsers()
	output, err := FindUser(db, email)
	if err != nil {
		panic(err)
	}
	if output.Email != email {
		t.Error("Emails did not match round trip")
	}
}

func TestReadOneSingleOutput(t *testing.T) {
	email := "test@dummyemail.com"
	err := AddUser(db, "test@dummyemail.com")
	if err != nil {
		panic(err)
	}
	defer deleteAllUsers()
	foundEmail, err := FindUserEmail(db, email)
	if err != nil {
		panic(err)
	}
	if *foundEmail != email {
		t.Error("Emails did not match")
	}
}

func TestReadOneSingleOutputWhenNotFound(t *testing.T) {
	email := "test@dummyemail.com"
	_, err := FindUserEmail(db, email)
	if err == nil {
		t.Error("Should have an error because this user does not exist")
	}
}

func TestMultiReadNoModel(t *testing.T) {
	// keep sorted
	emails := []string{"a@a.com", "b@b.com"}
	for _, e := range emails {
		err := AddUser(db, e)
		if err != nil {
			panic(err)
		}
	}
	defer deleteAllUsers()
	userlist, err := GetUserListNoModel(db)
	if err != nil {
		panic(err)
	}
	if len(userlist) != len(emails) {
		t.Error("Did not find all emails")
	}
	for ix, e := range emails {
		if e != userlist[ix].Email {
			t.Error("Emails did not match")
		}
	}
}

func TestMultiReadWithModel(t *testing.T) {
	// keep sorted
	emails := []string{"a@a.com", "b@b.com"}
	for _, e := range emails {
		err := AddUser(db, e)
		if err != nil {
			panic(err)
		}
	}
	defer deleteAllUsers()
	userlist, err := GetUserListWithModel(db)
	if err != nil {
		panic(err)
	}
	if len(userlist) != len(emails) {
		t.Error("Did not find all emails")
	}
	for ix, e := range emails {
		if e != userlist[ix].Email {
			t.Error("Emails did not match")
		}
	}
}

func TestMultiReadSingleColumnNoModel(t *testing.T) {
	// keep sorted
	emails := []string{"a@a.com", "b@b.com"}
	for _, e := range emails {
		err := AddUser(db, e)
		if err != nil {
			panic(err)
		}
	}
	defer deleteAllUsers()
	userlist, err := GetUserEmailsNoModel(db)
	if err != nil {
		panic(err)
	}
	if len(userlist) != len(emails) {
		t.Error("Did not find all emails")
	}
	for ix, e := range emails {
		if e != userlist[ix] {
			t.Error("Emails did not match")
		}
	}
}
