// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

package main

import (
	"bytes"
	"fmt"
	"go/build"
	"go/doc"
	"go/parser"
	"go/token"
	"log"
	"os"
	"text/template"
)

func main() {
	pkg, err := build.Import("robpike.io/ivy", "", build.ImportComment)
	if err != nil {
		log.Fatal(err)
	}
	fs := token.NewFileSet()
	pkgs, err := parser.ParseDir(fs, pkg.Dir, nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}
	astPkg := pkgs[pkg.Name]
	if astPkg == nil {
		log.Fatalf("failed to locate %s package", pkg.Name)
	}

	docPkg := doc.New(astPkg, pkg.ImportPath, doc.AllDecls)

	buf := new(bytes.Buffer)
	fmt.Fprintln(buf, `<!-- auto-generated from robpike.io/ivy package doc -->`)
	fmt.Fprintln(buf, head)
	fmt.Fprintln(buf, `<body>`)
	doc.ToHTML(buf, docPkg.Doc, nil)
	fmt.Fprintln(buf, `</body></html>`)

	tmpl.Execute(os.Stdout, string(bytes.Replace(buf.Bytes(), []byte{'`'}, []byte{'"'}, -1)))
}

var tmpl = template.Must(template.New("help.go").Parse("package mobile\n// GENERATED; DO NOT EDIT\n\nconst help = `{{.}}`\n"))

const head = `
<head>
    <style>
        body {
                font-family: Arial, sans-serif;
	        font-size: 10pt;
                line-height: 1.3em;
                max-width: 950px;
                word-break: normal;
                word-wrap: normal;
        }

        pre {
                border-radius: 10px;
                border: 2px solid #8AC007;
		font-family: monospace;
		font-size: 10pt;
                overflow: auto;
                padding: 10px;
                white-space: pre;
        }
    </style>
</head>`
