# norm
Golang DB layer generator without needing an intermediate ORM layer. It allows
you to write SQL, with some comment annotations. These annotations are used to
generate the Golang code.

# Example
This example uses PostgreSQL, but this should work with other databases too.

Suppose you have the following SQL query which selects a given user from a
`user` table. 

```sql
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
```

`norm` will generate the following API for the above declaration. Note that it
allows you to do a custom bind by generating low-level scanning code. In
addition, it provides an optional convenience method that returns a wrapper
struct for the output.

```go
type GetUserListNoModelResult struct {
	stmt *sql.Stmt
	rows *sql.Rows
}

func (res GetUserListNoModelResult) Next() bool {
	return res.rows.Next()
}

func (res GetUserListNoModelResult) Scan(ID *int, Email **string) error {
	return res.rows.Scan(ID, Email)
}

func (res GetUserListNoModelResult) Close() {
	if res.rows != nil {
		res.rows.Close()
	}
	if res.stmt != nil {
		res.stmt.Close()
	}
}

// Retrieves all emails from the users table
func (n *Norm) GetUserListNoModelScan(limit int, offset int) (*GetUserListNoModelResult, error) {
	result := GetUserListNoModelResult{}
	var err error
	result.stmt, err = n.db.Prepare(`SELECT id, email
FROM users
LIMIT $1
OFFSET $2`)
	if err != nil {
		return nil, err
	}
	result.rows, err = result.stmt.Query(limit, offset)
	if err != nil {
		defer result.stmt.Close()
		return nil, err
	}
	return &result, nil
}

type GetUserListNoModelOutput struct {
	ID    int
	Email *string
}

func (n *Norm) GetUserListNoModel(limit int, offset int) ([]GetUserListNoModelOutput, error) {
	res, err := n.GetUserListNoModelScan(limit, offset)
	if err != nil {
		return nil, err
	}
	defer res.Close()
	var ret []GetUserListNoModelOutput
	for res.Next() {
		var o GetUserListNoModelOutput
		if err := res.Scan(&o.ID, &o.Email); err != nil {
			return ret, err
		}
		ret = append(ret, o)
	}
	return ret, nil
}

```
