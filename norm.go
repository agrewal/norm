/**
`norm` can be used with `go generate` to create a simple API for programs. It
does not force a object structure, which can be decided outside of this layer.
This allows consumers to not have leaky DB related fluff in their models.

This executable must be called with one argument - the input file.

Todo:
- Create a backup and restore when this command fails
*/
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/format"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"text/template"
	"time"
)

const header = `// Code generated by norm. DO NOT EDIT.
// Generated on: {{.date}}
package {{.package}}

import (
	"database/sql"
	_ "{{.driverLib}}"
	{{.imports}}
)

type Norm struct {
	db *sql.DB
}

func NewNorm(connStr string) (*Norm, error) {
	db, err := sql.Open("{{.driverName}}", connStr)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return &Norm{db}, nil
}

func (n *Norm) Close() {
	n.db.Close()
}
`

var headerTmpl *template.Template

const readOne = `
{{if .Model}}
{{range .Doc}}// {{print .}}{{end}}
func (n *Norm) {{.FuncName}}({{getFuncSig .Inputs}}) (*{{.Model}}, error) {
    {{range .Outputs}}
	var _internal_{{.Name}} {{.Typ}}
	{{end}}
	stmt, err := n.db.Prepare(` + "`{{.BodyString}}`" + `)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	if err = stmt.QueryRow({{getCallSig .Inputs}}).Scan({{getCallSigWithPrefix .Outputs "&_internal_"}}); err != nil {
		return nil, err
	}
	return &{{.Model}}{
		{{range .Outputs}}
		{{.Name}}: _internal_{{.Name}},
		{{end}}
	}, nil
}
{{else}}
{{range .Doc}}// {{print .}}{{end}}
func (n *Norm) {{.FuncName}}({{if .Inputs}}{{getFuncSig .Inputs}}, {{end}}{{getFuncSigWithTypePrefix .Outputs "*"}}) error {
	stmt, err := n.db.Prepare(` + "`{{.BodyString}}`" + `)
	if err != nil {
		return err
	}
	defer stmt.Close()
	if err = stmt.QueryRow({{getCallSig .Inputs}}).Scan({{getCallSig .Outputs}}); err != nil {
		return err
	}
	return nil
}
{{end}}
`

var readOneTmpl *template.Template

const read = `
type {{.FuncName}}Result struct {
	stmt *sql.Stmt
	rows *sql.Rows
}

func (res {{.FuncName}}Result) Next() bool {
	return res.rows.Next()
}

func (res {{.FuncName}}Result) Scan({{getFuncSigWithTypePrefix .Outputs "*"}}) error {
	return res.rows.Scan({{getCallSig .Outputs}})
}

func (res {{.FuncName}}Result) Close() {
	if (res.rows != nil) {
		res.rows.Close()
	}
	if (res.stmt != nil) {
		res.stmt.Close()
	}
}

{{range .Doc}}// {{print .}}{{end}}
func (n *Norm) {{.FuncName}}Scan({{getFuncSig .Inputs}}) (*{{.FuncName}}Result, error) {
	result := {{.FuncName}}Result{}
	var err error
	result.stmt, err = n.db.Prepare(` + "`{{.BodyString}}`" + `)
	if err != nil {
		return nil, err
	}
	result.rows, err = result.stmt.Query({{getCallSig .Inputs}})
	if err != nil {
		defer result.stmt.Close()
		return nil, err
	}
	return &result, nil
}

{{if .Model}}
func (n *Norm) {{.FuncName}}({{getFuncSig .Inputs}}) ([]{{.Model}}, error) {
	res, err := n.{{.FuncName}}Scan({{getCallSig .Inputs}})
	if (err != nil) {
		return nil, err
	}
	defer res.Close()
	var ret []{{.Model}}
	for res.Next() {
		var o {{.Model}}
		if err := res.Scan({{getCallSigWithPrefix .Outputs "&o."}}); err != nil {
			return ret, err
		}
		ret = append(ret, o)
	}
	return ret, nil
}
{{else}}
type {{.FuncName}}Output struct {
{{getStructSig .Outputs}}
}

func (n *Norm) {{.FuncName}}({{getFuncSig .Inputs}}) ([]{{.FuncName}}Output, error) {
	res, err := n.{{.FuncName}}Scan({{getCallSig .Inputs}})
	if (err != nil) {
		return nil, err
	}
	defer res.Close()
	var ret []{{.FuncName}}Output
	for res.Next() {
		var o {{.FuncName}}Output
		if err := res.Scan({{getCallSigWithPrefix .Outputs "&o."}}); err != nil {
			return ret, err
		}
		ret = append(ret, o)
	}
	return ret, nil
}
{{end}}
`

var readTmpl *template.Template

const exec = `
{{range .Doc}}// {{print .}}{{end}}
func (n *Norm) {{.FuncName}}({{getFuncSig .Inputs}}) error {
	stmt, err := n.db.Prepare(` + "`{{.BodyString}}`" + `)
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec({{getCallSig .Inputs}})
	if err != nil {
		return err
	}
	return nil
}
`

var execTmpl *template.Template

var funcMap template.FuncMap = template.FuncMap{
	"getFuncSig":               getFuncSig,
	"getFuncSigWithTypePrefix": getFuncSigWithTypePrefix,
	"getCallSig":               getCallSig,
	"getCallSigWithPrefix":     getCallSigWithPrefix,
	"getStructSig":             getStructSig,
}

type genAble interface {
	gen(io.Writer) error
}

type arg struct {
	Name string
	Typ  string
}

type cmdBase struct {
	FuncName string
	Inputs   []arg
	Outputs  []arg
	Doc      []string
	Body     []string
	Model    *string
}

func (c *cmdBase) BodyString() string {
	return strings.Join(c.Body, "\n")
}

type cmdReadOne struct {
	cmdBase
}

func (c *cmdReadOne) gen(w io.Writer) error {
	return readOneTmpl.Execute(w, c)
}

type cmdRead struct {
	cmdBase
}

func (c *cmdRead) gen(w io.Writer) error {
	return readTmpl.Execute(w, c)
}

type cmdExec struct {
	cmdBase
}

func (c *cmdExec) gen(w io.Writer) error {
	return execTmpl.Execute(w, c)
}

func checkStart(firstLine string) bool {
	return firstLine == "-- !norm"
}

func getFuncSig(args []arg) string {
	var ret strings.Builder
	for ix, a := range args {
		if ix < len(args)-1 {
			fmt.Fprintf(&ret, "%s %s, ", a.Name, a.Typ)
		} else {
			fmt.Fprintf(&ret, "%s %s", a.Name, a.Typ)
		}
	}
	return ret.String()
}

func getFuncSigWithTypePrefix(args []arg, typPrefix string) string {
	var ret strings.Builder
	for ix, a := range args {
		if ix < len(args)-1 {
			fmt.Fprintf(&ret, "%s %s%s, ", a.Name, typPrefix, a.Typ)
		} else {
			fmt.Fprintf(&ret, "%s %s%s", a.Name, typPrefix, a.Typ)
		}
	}
	return ret.String()
}

func getCallSig(args []arg) string {
	var ret strings.Builder
	for ix, a := range args {
		if ix < len(args)-1 {
			fmt.Fprintf(&ret, "%s, ", a.Name)
		} else {
			fmt.Fprintf(&ret, "%s", a.Name)
		}
	}
	return ret.String()
}

func getCallSigWithPrefix(args []arg, prefix string) string {
	var ret strings.Builder
	for ix, a := range args {
		if ix < len(args)-1 {
			fmt.Fprintf(&ret, "%s%s, ", prefix, a.Name)
		} else {
			fmt.Fprintf(&ret, "%s%s", prefix, a.Name)
		}
	}
	return ret.String()
}

func getStructSig(args []arg) string {
	var ret strings.Builder
	for ix, a := range args {
		if ix < len(args)-1 {
			fmt.Fprintf(&ret, "\t%s %s\n", a.Name, a.Typ)
		} else {
			fmt.Fprintf(&ret, "\t%s %s", a.Name, a.Typ)
		}
	}
	return ret.String()

}

func main() {
	args := os.Args[1:]
	if len(args) != 1 {
		panic("Need exactly one argument to program")
	}
	inputFile := args[0]

	f, err := os.Open(inputFile)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	var gens []genAble
	var imports []string
	scanner := bufio.NewScanner(f)
	outFile := "db.go"
	pkgName := "db"
	driverLib := "github.com/lib/pq"
	driverName := "postgres"
	i := 1

	rxFile := regexp.MustCompile(`^-- !file ([^\s]+)$`)
	rxPkg := regexp.MustCompile(`^-- !package ([^\s]+)$`)
	rxImports := regexp.MustCompile(`^-- !import (.+)$`)
	rxDriverLib := regexp.MustCompile(`^-- !driver_lib ([^\s]+)$`)
	rxDriverName := regexp.MustCompile(`^-- !driver_name ([^\s]+)$`)
	rxReadOne := regexp.MustCompile(`^-- !read_one ([^\s]+)$`)
	rxRead := regexp.MustCompile(`^-- !read ([^\s]+)$`)
	rxExec := regexp.MustCompile(`^-- !exec ([^\s]+)$`)
	rxInput := regexp.MustCompile(`^-- !input ([^\s]+) ([^\s]+)$`)
	rxOutput := regexp.MustCompile(`^-- !output ([^\s]+) ([^\s]+)$`)
	rxModel := regexp.MustCompile(`^-- !model ([^\s]+)$`)
	rxDoc := regexp.MustCompile(`^-- !doc (.+)`)

	for scanner.Scan() {
		line := scanner.Text()
		if i == 1 && !checkStart(line) {
			panic("Not a valid norm file")
		} else if i == 1 {
			i++
			continue
		}
		if strings.HasPrefix(line, `-- !file`) {
			matches := rxFile.FindStringSubmatch(line)
			if len(matches) != 2 {
				panic(fmt.Sprintf("Format error on line %d: %q", i, line))
			}
			outFile = matches[1]
			i++
			continue
		}
		if strings.HasPrefix(line, `-- !package`) {
			matches := rxPkg.FindStringSubmatch(line)
			if len(matches) != 2 {
				panic(fmt.Sprintf("Format error on line %d: %q", i, line))
			}
			pkgName = matches[1]
			i++
			continue
		}
		if strings.HasPrefix(line, `-- !driver_lib`) {
			matches := rxDriverLib.FindStringSubmatch(line)
			if len(matches) != 2 {
				panic(fmt.Sprintf("Format error on line %d: %q", i, line))
			}
			driverLib = matches[1]
			i++
			continue
		}
		if strings.HasPrefix(line, `-- !import`) {
			matches := rxImports.FindStringSubmatch(line)
			if len(matches) != 2 {
				panic(fmt.Sprintf("Format error on line %d: %q", i, line))
			}
			imports = append(imports, matches[1])
			i++
			continue
		}
		if strings.HasPrefix(line, `-- !driver_name`) {
			matches := rxDriverName.FindStringSubmatch(line)
			if len(matches) != 2 {
				panic(fmt.Sprintf("Format error on line %d: %q", i, line))
			}
			driverName = matches[1]
			i++
			continue
		}
		if strings.HasPrefix(line, `-- !read_one`) {
			matches := rxReadOne.FindStringSubmatch(line)
			if len(matches) != 2 {
				panic(fmt.Sprintf("Format error on line %d: %q", i, line))
			}
			cmd := &cmdReadOne{}
			gens = append(gens, cmd)
			cmd.FuncName = matches[1]
			i++
			// Start sub-scan
			for scanner.Scan() {
				line := scanner.Text()
				if len(strings.TrimSpace(line)) == 0 {
					i++
					break
				}
				if strings.HasPrefix(line, `-- !input`) {
					matches := rxInput.FindStringSubmatch(line)
					if len(matches) != 3 {
						panic(fmt.Sprintf("Format error on line %d: %q", i, line))
					}
					inp := arg{matches[1], matches[2]}
					cmd.Inputs = append(cmd.Inputs, inp)
					i++
					continue
				}
				if strings.HasPrefix(line, `-- !output`) {
					matches := rxOutput.FindStringSubmatch(line)
					if len(matches) != 3 {
						panic(fmt.Sprintf("Format error on line %d: %q", i, line))
					}
					out := arg{matches[1], matches[2]}
					cmd.Outputs = append(cmd.Outputs, out)
					i++
					continue
				}
				if strings.HasPrefix(line, `-- !doc`) {
					matches := rxDoc.FindStringSubmatch(line)
					if len(matches) != 2 {
						panic(fmt.Sprintf("Format error on line %d: %q", i, line))
					}
					cmd.Doc = append(cmd.Doc, matches[1])
					i++
					continue
				}
				if strings.HasPrefix(line, `-- !model`) {
					matches := rxModel.FindStringSubmatch(line)
					if len(matches) != 2 {
						panic(fmt.Sprintf("Format error on line %d: %q", i, line))
					}
					cmd.Model = &matches[1]
					i++
					continue
				}
				if strings.HasPrefix(line, `-- !`) {
					panic(fmt.Sprintf("Unknown command on line %d: %q", i, line))
				}
				cmd.Body = append(cmd.Body, line)
				i++
			}
			continue
		}
		if strings.HasPrefix(line, `-- !read `) {
			matches := rxRead.FindStringSubmatch(line)
			if len(matches) != 2 {
				panic(fmt.Sprintf("Format error on line %d: %q", i, line))
			}
			cmd := &cmdRead{}
			gens = append(gens, cmd)
			cmd.FuncName = matches[1]
			i++
			// Start sub-scan
			for scanner.Scan() {
				line := scanner.Text()
				if len(strings.TrimSpace(line)) == 0 {
					i++
					break
				}
				if strings.HasPrefix(line, `-- !input`) {
					matches := rxInput.FindStringSubmatch(line)
					if len(matches) != 3 {
						panic(fmt.Sprintf("Format error on line %d: %q", i, line))
					}
					inp := arg{matches[1], matches[2]}
					cmd.Inputs = append(cmd.Inputs, inp)
					i++
					continue
				}
				if strings.HasPrefix(line, `-- !output`) {
					matches := rxOutput.FindStringSubmatch(line)
					if len(matches) != 3 {
						panic(fmt.Sprintf("Format error on line %d: %q", i, line))
					}
					out := arg{matches[1], matches[2]}
					cmd.Outputs = append(cmd.Outputs, out)
					i++
					continue
				}
				if strings.HasPrefix(line, `-- !doc`) {
					matches := rxDoc.FindStringSubmatch(line)
					if len(matches) != 2 {
						panic(fmt.Sprintf("Format error on line %d: %q", i, line))
					}
					cmd.Doc = append(cmd.Doc, matches[1])
					i++
					continue
				}
				if strings.HasPrefix(line, `-- !model`) {
					matches := rxModel.FindStringSubmatch(line)
					if len(matches) != 2 {
						panic(fmt.Sprintf("Format error on line %d: %q", i, line))
					}
					cmd.Model = &matches[1]
					i++
					continue
				}
				if strings.HasPrefix(line, `-- !`) {
					panic(fmt.Sprintf("Unknown command on line %d: %q", i, line))
				}
				cmd.Body = append(cmd.Body, line)
				i++
			}
			continue
		}
		if strings.HasPrefix(line, `-- !exec`) {
			matches := rxExec.FindStringSubmatch(line)
			if len(matches) != 2 {
				panic(fmt.Sprintf("Format error on line %d: %q", i, line))
			}
			cmd := &cmdExec{}
			gens = append(gens, cmd)
			cmd.FuncName = matches[1]
			i++
			// Start sub-scan
			for scanner.Scan() {
				line := scanner.Text()
				if len(strings.TrimSpace(line)) == 0 {
					i++
					break
				}
				if strings.HasPrefix(line, `-- !input`) {
					matches := rxInput.FindStringSubmatch(line)
					if len(matches) != 3 {
						panic(fmt.Sprintf("Format error on line %d: %q", i, line))
					}
					inp := arg{matches[1], matches[2]}
					cmd.Inputs = append(cmd.Inputs, inp)
					i++
					continue
				}
				if strings.HasPrefix(line, `-- !doc`) {
					matches := rxDoc.FindStringSubmatch(line)
					if len(matches) != 2 {
						panic(fmt.Sprintf("Format error on line %d: %q", i, line))
					}
					cmd.Doc = append(cmd.Doc, matches[1])
					i++
					continue
				}
				if strings.HasPrefix(line, `-- !`) {
					panic(fmt.Sprintf("Unknown command on line %d: %q", i, line))
				}
				cmd.Body = append(cmd.Body, line)
				i++
			}
			continue
		}
		if strings.HasPrefix(line, `-- !`) {
			panic(fmt.Sprintf("Unknown command on line %d: %q", i, line))
		}
		i++
	}

	var bb bytes.Buffer

	headerTmpl, err = template.New("header").Parse(header)
	if err != nil {
		panic(err)
	}
	readOneTmpl, err = template.New("read_one").Funcs(funcMap).Parse(readOne)
	if err != nil {
		panic(err)
	}
	readTmpl, err = template.New("read").Funcs(funcMap).Parse(read)
	if err != nil {
		panic(err)
	}
	execTmpl, err = template.New("exec").Funcs(funcMap).Parse(exec)
	if err != nil {
		panic(err)
	}

	// do writes
	if err = headerTmpl.Execute(&bb, map[string]string{
		"package":    pkgName,
		"date":       fmt.Sprintf("%s", time.Now()),
		"driverLib":  driverLib,
		"driverName": driverName,
		"imports":    strings.Join(imports, "\n"),
	}); err != nil {
		panic(err)
	}

	for _, cmd := range gens {
		if err = cmd.gen(&bb); err != nil {
			panic(err)
		}
	}

	unformatted := bb.Bytes()
	formatted, err := format.Source(unformatted)
	if err != nil {
		panic(err)
	}

	err = ioutil.WriteFile(outFile, formatted, 0644)
	if err != nil {
		panic(err)
	}
}
