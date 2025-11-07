//go:build ignore
// +build ignore

package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Lightweight case YAML structures
type RequestSpec struct {
	Method  string                 `yaml:"method"`
	Path    string                 `yaml:"path"`
	Headers map[string]string      `yaml:"headers,omitempty"`
	Body    map[string]interface{} `yaml:"body,omitempty"`
}

type ExpectSpec struct {
	Status int                    `yaml:"status"`
	Body   map[string]interface{} `yaml:"body,omitempty"`
}

type CaseYAML struct {
	Name    string      `yaml:"name"`
	Request RequestSpec `yaml:"request"`
	Expect  ExpectSpec  `yaml:"expect"`
}

// Helper: write YAML file
func writeCaseYAML(path string, c CaseYAML) error {
	b, err := yaml.Marshal(&c)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return ioutil.WriteFile(path, b, 0o644)
}

// AST-based extractor
func findFunctionAndTypes(rootDir, target string) (funcFile string, funcDecl *ast.FuncDecl, typeFields map[string][]string, fset *token.FileSet, err error) {
	// typeFields maps typeName -> field json names
	typeFields = map[string][]string{}
	fset = token.NewFileSet()
	// support target forms: "Func" or "Receiver.Func"
	recvName := ""
	funcName := target
	if strings.Contains(target, ".") {
		parts := strings.SplitN(target, ".", 2)
		recvName = parts[0]
		funcName = parts[1]
	}

	// walk files
	err = filepath.Walk(rootDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if info.IsDir() {
			// skip vendor and test data
			if info.Name() == "vendor" || info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		// parse file
		f, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if perr != nil {
			return nil
		}
		// collect type structs
		for _, decl := range f.Decls {
			gdecl, ok := decl.(*ast.GenDecl)
			if !ok || gdecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range gdecl.Specs {
				tspec := spec.(*ast.TypeSpec)
				if st, ok := tspec.Type.(*ast.StructType); ok {
					fields := []string{}
					for _, f := range st.Fields.List {
						// determine json tag or field name
						name := ""
						if f.Tag != nil {
							t := strings.Trim(f.Tag.Value, "`")
							// find json:"..."
							for _, part := range strings.Split(t, " ") {
								if strings.HasPrefix(part, "json:") {
									jsonTag := strings.TrimPrefix(part, "json:")
									jsonTag = strings.Trim(jsonTag, `"`)
									if jsonTag != "-" && jsonTag != "" {
										name = strings.Split(jsonTag, ",")[0]
									}
								}
							}
						}
						if name == "" && len(f.Names) > 0 {
							name = strings.ToLower(f.Names[0].Name)
						}
						if name != "" {
							fields = append(fields, name)
						}
					}
					if len(fields) > 0 {
						typeFields[tspec.Name.Name] = fields
					}
				}
			}
		}
		// find function decl
		for _, decl := range f.Decls {
			if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name != nil && fd.Name.Name == funcName {
				// if receiver specified, check receiver type
				if recvName != "" {
					if fd.Recv == nil || len(fd.Recv.List) == 0 {
						continue
					}
					// receiver type could be *Server or Server
					recvType := ""
					switch expr := fd.Recv.List[0].Type.(type) {
					case *ast.StarExpr:
						if id, ok := expr.X.(*ast.Ident); ok {
							recvType = id.Name
						}
					case *ast.Ident:
						recvType = expr.Name
					}
					if recvType != recvName {
						continue
					}
				}
				// found
				funcFile = path
				funcDecl = fd
				return filepath.SkipDir // stop walking
			}
		}
		return nil
	})
	if funcDecl == nil {
		return "", nil, typeFields, fset, fmt.Errorf("function %s not found", target)
	}
	return funcFile, funcDecl, typeFields, fset, nil
}

// inspect function AST to find request struct variable bound by ShouldBindJSON/BindJSON and response gin.H keys
func inspectFunc(fd *ast.FuncDecl, typeFields map[string][]string) (requestFields []string, responseKeys []string, routePath string) {
	// traverse body and look for call expressions
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		ce, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// check function is a selector expression (e.g., c.ShouldBindJSON or c.JSON)
		sel, isSel := ce.Fun.(*ast.SelectorExpr)
		if !isSel {
			return true
		}

		mname := sel.Sel.Name

		// 1) Request binding: ShouldBindJSON/BindJSON/ShouldBind/Bind
		if mname == "ShouldBindJSON" || mname == "BindJSON" || mname == "ShouldBind" || mname == "Bind" {
			if len(ce.Args) >= 1 {
				// expect first arg like &var
				if ua, ok := ce.Args[0].(*ast.UnaryExpr); ok {
					if id, ok := ua.X.(*ast.Ident); ok {
						vname := id.Name
						// find vname declaration or assignment in function body
						var vType string
						ast.Inspect(fd.Body, func(n2 ast.Node) bool {
							// value spec: var vname Type
							if vs, ok := n2.(*ast.ValueSpec); ok {
								for _, nm := range vs.Names {
									if nm.Name == vname {
										if vs.Type != nil {
											if idt, ok := vs.Type.(*ast.Ident); ok {
												vType = idt.Name
											}
										}
									}
								}
							}
							// assign stmt: vname := Type{}
							if as, ok := n2.(*ast.AssignStmt); ok {
								for i, lhs := range as.Lhs {
									if ident, ok := lhs.(*ast.Ident); ok && ident.Name == vname {
										if len(as.Rhs) > i {
											if cl, ok := as.Rhs[i].(*ast.CompositeLit); ok {
												if idt, ok := cl.Type.(*ast.Ident); ok {
													vType = idt.Name
												}
											}
										}
									}
								}
							}
							return true
						})
						// if vType known, append its fields
						if vType != "" {
							if flds, ok := typeFields[vType]; ok {
								requestFields = append(requestFields, flds...)
							}
						}
					}
				}
			}
		}

		// 2) Response JSON: c.JSON(status, gin.H{...})
		if mname == "JSON" {
			if len(ce.Args) >= 2 {
				if cl, ok := ce.Args[1].(*ast.CompositeLit); ok {
					if se, ok := cl.Type.(*ast.SelectorExpr); ok {
						if ident, ok := se.X.(*ast.Ident); ok && ident.Name == "gin" && se.Sel.Name == "H" {
							for _, e := range cl.Elts {
								if kv, ok := e.(*ast.KeyValueExpr); ok {
									switch k := kv.Key.(type) {
									case *ast.BasicLit:
										key := strings.Trim(k.Value, `"`)
										responseKeys = append(responseKeys, key)
									case *ast.Ident:
										responseKeys = append(responseKeys, k.Name)
									}
								}
							}
						}
					}
				}
			}
		}

		return true
	})

	// detect route via AST scanning
	routePath = detectRouteForFunc(fd.Name.Name)
	return
}

// detectRouteForFunc uses simple AST search for calls like r.POST("/path", handler)
// It also attempts to resolve a group prefix when the router variable comes from a Group("/prefix") call.
func detectRouteForFunc(funcName string) string {
	fset := token.NewFileSet()
	var found string
	_ = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return nil
		}
		// map of identifier -> group prefix (if assigned by .Group("/prefix"))
		prefixMap := map[string]string{}
		ast.Inspect(f, func(n ast.Node) bool {
			// look for assignments like api := router.Group("/api") or api := s.router.Group("/api")
			if as, ok := n.(*ast.AssignStmt); ok {
				for i, rhs := range as.Rhs {
					if ce, ok := rhs.(*ast.CallExpr); ok {
						if sel, ok := ce.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Group" {
							// first arg string literal
							if len(ce.Args) >= 1 {
								if bl, ok := ce.Args[0].(*ast.BasicLit); ok {
									prefix := strings.Trim(bl.Value, `"`)
									if i < len(as.Lhs) {
										if id, ok := as.Lhs[i].(*ast.Ident); ok {
											prefixMap[id.Name] = prefix
										}
									}
								}
							}
						}
					}
				}
			}
			return true
		})

		ast.Inspect(f, func(n ast.Node) bool {
			ce, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			// fun could be SelectorExpr like api.POST
			if sel, ok := ce.Fun.(*ast.SelectorExpr); ok {
				m := sel.Sel.Name
				if m == "POST" || m == "GET" || m == "PUT" || m == "DELETE" || m == "PATCH" || m == "Any" {
					// first arg maybe path
					if len(ce.Args) >= 1 {
						if bl, ok := ce.Args[0].(*ast.BasicLit); ok && bl.Kind == token.STRING {
							pathStr := strings.Trim(bl.Value, `"`)
							// check if handler name appears among args
							for _, a := range ce.Args[1:] {
								s := fmt.Sprint(a)
								if strings.Contains(s, funcName) {
									// resolve prefix if selector receiver is an identifier bound to Group
									prefix := ""
									if id, ok := sel.X.(*ast.Ident); ok {
										if p, ok2 := prefixMap[id.Name]; ok2 {
											prefix = p
										}
									}
									if prefix != "" && strings.HasPrefix(pathStr, "/") {
										// combine and ensure single slash
										returnVal := strings.TrimRight(prefix, "/") + pathStr
										found = returnVal
									} else {
										found = pathStr
									}
									return false
								}
							}
						}
					}
				}
			}
			return true
		})
		if found != "" {
			return fmt.Errorf("stopwalk")
		}
		return nil
	})
	return found
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: go run %s <Receiver.Optional>.<Func> or <Func> [caseName]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
		os.Exit(1)
	}
	target := args[0]
	caseName := "case01"
	if len(args) >= 2 {
		caseName = args[1]
	}

	root := "."
	funcFile, fd, typeFields, _, err := findFunctionAndTypes(root, target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
	fmt.Printf("found function in %s\n", funcFile)

	reqFields, respKeys, route := inspectFunc(fd, typeFields)

	// build CaseYAML
	req := RequestSpec{Method: "POST", Path: "", Headers: map[string]string{}, Body: map[string]interface{}{}}
	if route != "" {
		req.Path = route
	} else {
		req.Path = "/api/" + fd.Name.Name
	}
	for _, k := range reqFields {
		req.Body[k] = ""
	}
	if len(req.Body) == 0 {
		// create a placeholder because sometimes we couldn't infer
		req.Body["example_field"] = ""
	}

	exp := ExpectSpec{Status: 200, Body: map[string]interface{}{}}
	for _, k := range respKeys {
		// heuristics
		if strings.Contains(strings.ToLower(k), "token") || strings.Contains(strings.ToLower(k), "id") {
			exp.Body[k] = "EXISTS:true"
		} else {
			exp.Body[k] = "*"
		}
	}
	if len(exp.Body) == 0 {
		exp.Body["example_response_field"] = "*"
	}

	cy := CaseYAML{
		Name:    fd.Name.Name + " " + caseName,
		Request: req,
		Expect:  exp,
	}
	outPath := filepath.Join("test", fd.Name.Name, caseName, "case.yml")
	if err := writeCaseYAML(outPath, cy); err != nil {
		fmt.Fprintf(os.Stderr, "write case.yml failed: %v\n", err)
		os.Exit(3)
	}
	// ensure PrepareData/CheckData dirs exist
	_ = os.MkdirAll(filepath.Join("test", fd.Name.Name, caseName, "PrepareData"), 0o755)
	_ = os.MkdirAll(filepath.Join("test", fd.Name.Name, caseName, "CheckData"), 0o755)

	fmt.Printf("generated %s\n", outPath)

	// Generate harness-style test wrapper under parent test/<Func>/ (not per-case)
	parentTestDir := filepath.Join("test", fd.Name.Name)
	if err := os.MkdirAll(parentTestDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create parent test dir failed: %v\n", err)
		os.Exit(4)
	}
	testFilePath := filepath.Join(parentTestDir, fd.Name.Name+"_test.go")
	f2, err := os.Create(testFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create test file failed: %v\n", err)
		os.Exit(5)
	}
	defer f2.Close()

	// write harness-style test in parent dir
	pkgName := strings.ToLower(fd.Name.Name)

	// Build a 'See:' navigation comment pointing to the implementation file and signature
	relPath := funcFile
	if rp, rerr := filepath.Rel(".", funcFile); rerr == nil {
		relPath = rp
	}
	// assemble receiver string e.g. (s *Server)
	recvStr := ""
	if fd.Recv != nil && len(fd.Recv.List) > 0 {
		recvField := fd.Recv.List[0]
		recvName := "s"
		if len(recvField.Names) > 0 {
			recvName = recvField.Names[0].Name
		}
		// determine type name
		switch rt := recvField.Type.(type) {
		case *ast.StarExpr:
			if id, ok := rt.X.(*ast.Ident); ok {
				recvStr = "(" + recvName + " *" + id.Name + ")"
			} else if se, ok := rt.X.(*ast.SelectorExpr); ok {
				// selector like pkg.Type
				if xid, ok := se.X.(*ast.Ident); ok {
					recvStr = "(" + recvName + " *" + xid.Name + "." + se.Sel.Name + ")"
				}
			}
		case *ast.Ident:
			recvStr = "(" + recvName + " " + rt.Name + ")"
		default:
			// fallback
			recvStr = "(s *Server)"
		}
	}
	// assemble first parameter string e.g. (c *gin.Context)
	paramStr := "(c *gin.Context)"
	if fd.Type != nil && fd.Type.Params != nil && len(fd.Type.Params.List) > 0 {
		p := fd.Type.Params.List[0]
		pname := "c"
		if len(p.Names) > 0 {
			pname = p.Names[0].Name
		}
		s := ""
		swit := p.Type
		switch pt := swit.(type) {
		case *ast.StarExpr:
			if se, ok := pt.X.(*ast.SelectorExpr); ok {
				if xid, ok := se.X.(*ast.Ident); ok {
					s = "*" + xid.Name + "." + se.Sel.Name
				}
			} else if id, ok := pt.X.(*ast.Ident); ok {
				s = "*" + id.Name
			}
		case *ast.SelectorExpr:
			if xid, ok := pt.X.(*ast.Ident); ok {
				s = xid.Name + "." + pt.Sel.Name
			}
		case *ast.Ident:
			s = pt.Name
		default:
			s = "*gin.Context"
		}
		if s != "" {
			paramStr = "(" + pname + " " + s + ")"
		}
	}
	// write the See comment
	fmt.Fprintf(f2, "// See: %s -> func %s %s\n", filepath.ToSlash(relPath), recvStr, fd.Name.Name+paramStr)

	fmt.Fprintf(f2, "// @Target(%s)\n", fd.Name.Name)
	fmt.Fprintf(f2, "package %s\n\n", pkgName)
	fmt.Fprintln(f2, "import (")
	fmt.Fprintln(f2, "\t\"testing\"")
	fmt.Fprintln(f2, "\t\"nofx/test/harness\"")
	fmt.Fprintln(f2, ")\n")
	fmt.Fprintf(f2, "// %sTest 嵌入 BaseTest，可按需重写 Before/After 钩子\n", strings.Title(fd.Name.Name))
	fmt.Fprintf(f2, "type %sTest struct {\n\tharness.BaseTest\n}\n\n", strings.Title(fd.Name.Name))
	fmt.Fprintf(f2, "func (rt *%sTest) Before(t *testing.T) {\n\trt.BaseTest.Before(t)\n\tif rt.Env != nil {\n\t\tt.Logf(\"TestEnv API URL: %%s\", rt.Env.URL())\n\t} else {\n\t\tt.Log(\"Warning: Env is nil in Before\")\n\t}\n}\n\n", strings.Title(fd.Name.Name))
	fmt.Fprintf(f2, "func (rt *%sTest) After(t *testing.T) {\n\t// no-op\n}\n\n", strings.Title(fd.Name.Name))
	fmt.Fprintf(f2, "// @RunWith(%s)\nfunc Test%s(t *testing.T) {\n\trt := &%sTest{}\n\tharness.RunCase(t, rt)\n}\n", caseName, strings.Title(fd.Name.Name), strings.Title(fd.Name.Name))

	fmt.Printf("generated %s\n", testFilePath)
}
