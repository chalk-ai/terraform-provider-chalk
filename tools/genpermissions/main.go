// genpermissions analyzes the Terraform provider's resource implementations to
// automatically extract required gRPC permissions from proto annotations, then
// generates internal/provider/permissions_gen.go with a map that the provider
// uses at runtime to append permissions docs to each resource's schema description.
//
// It uses Go type-aware AST analysis to find service client calls in CRUD
// methods, then reads the chalk.auth.v1.permission and team_permission options
// directly from the embedded proto file descriptors — no manual mapping needed.
package main

import (
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	authv1 "github.com/chalk-ai/chalk-go/gen/chalk/auth/v1"
	// Import server proto packages to register their file descriptors.
	_ "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
)

func main() {
	providerDir := flag.String("provider-dir", ".", "path to the provider root directory")
	flag.Parse()

	rpcPerms := buildRPCPermMap()
	resPerms := analyzeProvider(*providerDir, rpcPerms)
	if err := generateGoFile(*providerDir, resPerms); err != nil {
		log.Fatalf("generating permissions file: %v", err)
	}
}

// permInfo holds the human-readable slug and scope for a single permission.
type permInfo struct {
	slug   string
	isTeam bool
}

// opPerms maps CRUD operation names to their required permissions.
type opPerms = map[string][]permInfo

// resourcePerms maps Terraform type names (e.g. "chalk_service_token") to their
// permissions. A name may appear twice — once as a resource and once as a data
// source — so the key is (tfName, isDataSource).
type resourcePerms = map[docKey]opPerms

type docKey struct {
	tfName       string
	isDataSource bool
}

// buildRPCPermMap ranges over all registered proto file descriptors and returns
// a map from RPC method name to its required permission.
func buildRPCPermMap() map[string]permInfo {
	result := make(map[string]permInfo)
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		for i := 0; i < fd.Services().Len(); i++ {
			svc := fd.Services().Get(i)
			for j := 0; j < svc.Methods().Len(); j++ {
				method := svc.Methods().Get(j)
				opts := method.Options()
				if opts == nil {
					continue
				}
				name := string(method.Name())
				switch {
				case proto.HasExtension(opts, authv1.E_Permission):
					perm := proto.GetExtension(opts, authv1.E_Permission).(authv1.Permission)
					result[name] = permInfo{slug: permSlug(perm), isTeam: false}
				case proto.HasExtension(opts, authv1.E_TeamPermission):
					perm := proto.GetExtension(opts, authv1.E_TeamPermission).(authv1.Permission)
					result[name] = permInfo{slug: permSlug(perm), isTeam: true}
				}
			}
		}
		return true
	})
	return result
}

// permSlug returns the human-readable slug for a Permission value (e.g.
// "infrastructure.write"), falling back to the enum name if the option is absent.
func permSlug(perm authv1.Permission) string {
	enumVal := perm.Descriptor().Values().ByNumber(protoreflect.EnumNumber(perm))
	if enumVal != nil {
		opts := enumVal.Options()
		if opts != nil && proto.HasExtension(opts, authv1.E_Slug) {
			return proto.GetExtension(opts, authv1.E_Slug).(string)
		}
	}
	return perm.String()
}

// analyzeProvider loads the internal/provider package and maps each Terraform
// resource/data-source type name to its per-operation RPC permissions.
func analyzeProvider(dir string, rpcPerms map[string]permInfo) resourcePerms {
	cfg := &packages.Config{
		Mode: packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedTypes | packages.NeedImports,
		Dir:  dir,
	}
	pkgs, err := packages.Load(cfg, "./internal/provider")
	if err != nil {
		log.Fatalf("loading packages: %v", err)
	}

	result := make(resourcePerms)
	for _, pkg := range pkgs {
		for _, e := range pkg.Errors {
			log.Printf("warning: package load error: %v", e)
		}
		analyzePackage(pkg, rpcPerms, result)
	}
	return result
}

// crudOps is the set of Terraform CRUD method names we care about.
var crudOps = map[string]bool{
	"Create": true,
	"Read":   true,
	"Update": true,
	"Delete": true,
}

// structMeta holds what we learn from a struct's method declarations.
type structMeta struct {
	tfName       string
	isDataSource bool
	opRPCs       map[string][]string // operation → []rpc method names
}

// analyzePackage inspects every function declaration in a package, recording
// Terraform type names (from Metadata methods) and RPC calls (from CRUD methods).
func analyzePackage(pkg *packages.Package, rpcPerms map[string]permInfo, result resourcePerms) {
	structs := make(map[string]*structMeta) // receiver type name → meta

	meta := func(recv string) *structMeta {
		if structs[recv] == nil {
			structs[recv] = &structMeta{opRPCs: make(map[string][]string)}
		}
		return structs[recv]
	}

	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 {
				continue
			}
			recv := receiverTypeName(fn.Recv)
			if recv == "" {
				continue
			}
			switch fn.Name.Name {
			case "Metadata":
				if name := extractTFTypeName(fn); name != "" {
					m := meta(recv)
					m.tfName = name
					m.isDataSource = isDataSourceMetadata(fn, pkg.TypesInfo)
				}
			default:
				if crudOps[fn.Name.Name] {
					rpcs := serviceClientCalls(fn, pkg.TypesInfo)
					if len(rpcs) > 0 {
						m := meta(recv)
						m.opRPCs[fn.Name.Name] = append(m.opRPCs[fn.Name.Name], rpcs...)
					}
				}
			}
		}
	}

	for _, m := range structs {
		if m.tfName == "" {
			continue
		}
		ops := make(opPerms)
		for op, rpcs := range m.opRPCs {
			seen := make(map[string]bool)
			for _, rpc := range rpcs {
				p, ok := rpcPerms[rpc]
				if !ok || isNonspecificPerm(p.slug) || seen[p.slug] {
					continue
				}
				ops[op] = append(ops[op], p)
				seen[p.slug] = true
			}
		}
		if len(ops) > 0 {
			result[docKey{m.tfName, m.isDataSource}] = ops
		}
	}
}

// isDataSourceMetadata returns true if the Metadata function belongs to a data
// source (its response parameter type is from the datasource package).
func isDataSourceMetadata(fn *ast.FuncDecl, info *types.Info) bool {
	if fn.Type.Params == nil {
		return false
	}
	for _, field := range fn.Type.Params.List {
		t := info.TypeOf(field.Type)
		if t == nil {
			continue
		}
		if pt, ok := t.(*types.Pointer); ok {
			t = pt.Elem()
		}
		named, ok := t.(*types.Named)
		if !ok || named.Obj().Pkg() == nil {
			continue
		}
		if strings.Contains(named.Obj().Pkg().Path(), "/datasource") {
			return true
		}
	}
	return false
}

// isNonspecificPerm returns true for permissions that are not meaningful to
// surface in docs (e.g. "just be authenticated").
func isNonspecificPerm(slug string) bool {
	switch slug {
	case "authenticated", "insecure", "unspecified":
		return true
	}
	return false
}

// receiverTypeName returns the type name of the first method receiver, e.g.
// "ServiceTokenResource" for `func (r *ServiceTokenResource) ...`.
func receiverTypeName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}
	t := recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	if ident, ok := t.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// extractTFTypeName extracts the Terraform type name from a Metadata method
// body, e.g. returns "chalk_service_token" for:
//
//	resp.TypeName = req.ProviderTypeName + "_service_token"
func extractTFTypeName(fn *ast.FuncDecl) string {
	var result string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		if len(assign.Lhs) != 1 {
			return true
		}
		sel, ok := assign.Lhs[0].(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "TypeName" {
			return true
		}
		result = extractConcatSuffix(assign.Rhs[0], "chalk")
		return true
	})
	return result
}

// extractConcatSuffix walks a binary + expression and returns the concatenated
// string, replacing any non-literal (selector/ident) parts with prefix.
// For `req.ProviderTypeName + "_service_token"` it returns "chalk_service_token".
func extractConcatSuffix(expr ast.Expr, prefix string) string {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			return strings.Trim(e.Value, `"`)
		}
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			left := extractConcatSuffix(e.X, prefix)
			right := extractConcatSuffix(e.Y, prefix)
			if left == "" {
				return prefix + right
			}
			return left + right
		}
	}
	return ""
}

// serviceClientCalls returns the names of all gRPC methods called directly on
// service client interfaces (types in the serverv1connect package) within fn.
func serviceClientCalls(fn *ast.FuncDecl, info *types.Info) []string {
	var calls []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		t := info.TypeOf(sel.X)
		if t == nil {
			return true
		}
		if pt, ok := t.(*types.Pointer); ok {
			t = pt.Elem()
		}
		if isServiceClientType(t) {
			calls = append(calls, sel.Sel.Name)
		}
		return true
	})
	return calls
}

// isServiceClientType returns true if t is a service client interface from the
// serverv1connect package (i.e. its name ends in "ServiceClient").
func isServiceClientType(t types.Type) bool {
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	if obj.Pkg() == nil {
		return false
	}
	return strings.HasSuffix(obj.Pkg().Path(), "/serverv1connect") &&
		strings.HasSuffix(obj.Name(), "ServiceClient")
}

// generateGoFile writes internal/provider/permissions_gen.go with two maps
// (one for resources, one for data sources) from Terraform type names to their
// required-permissions markdown string.
func generateGoFile(providerDir string, resPerms resourcePerms) error {
	type entry struct {
		tfName  string
		permDoc string
	}
	var resources, datasources []entry
	for key, ops := range resPerms {
		doc := buildPermDoc(ops)
		if doc == "" {
			continue
		}
		e := entry{key.tfName, doc}
		if key.isDataSource {
			datasources = append(datasources, e)
		} else {
			resources = append(resources, e)
		}
	}
	sort.Slice(resources, func(i, j int) bool { return resources[i].tfName < resources[j].tfName })
	sort.Slice(datasources, func(i, j int) bool { return datasources[i].tfName < datasources[j].tfName })

	var sb strings.Builder
	sb.WriteString("// Code generated by tools/genpermissions. DO NOT EDIT.\n")
	sb.WriteString("package provider\n\n")
	sb.WriteString("// resourcePermissionsMarkdown maps Terraform resource type names to their\n")
	sb.WriteString("// required-permissions documentation, appended to each schema's MarkdownDescription.\n")
	sb.WriteString("var resourcePermissionsMarkdown = map[string]string{\n")
	for _, e := range resources {
		sb.WriteString(fmt.Sprintf("\t%q: %q,\n", e.tfName, e.permDoc))
	}
	sb.WriteString("}\n\n")
	sb.WriteString("// datasourcePermissionsMarkdown maps Terraform data-source type names to their\n")
	sb.WriteString("// required-permissions documentation, appended to each schema's MarkdownDescription.\n")
	sb.WriteString("var datasourcePermissionsMarkdown = map[string]string{\n")
	for _, e := range datasources {
		sb.WriteString(fmt.Sprintf("\t%q: %q,\n", e.tfName, e.permDoc))
	}
	sb.WriteString("}\n")

	src, err := format.Source([]byte(sb.String()))
	if err != nil {
		return fmt.Errorf("formatting generated source: %w", err)
	}

	outPath := filepath.Join(providerDir, "internal", "provider", "permissions_gen.go")
	if err := os.WriteFile(outPath, src, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", outPath, err)
	}
	return nil
}

// buildPermDoc collects all unique permissions across operations and renders
// the inline markdown string.
func buildPermDoc(ops opPerms) string {
	seen := make(map[string]bool)
	var perms []permInfo
	for _, op := range []string{"Create", "Read", "Update", "Delete"} {
		for _, p := range ops[op] {
			if !seen[p.slug] {
				seen[p.slug] = true
				perms = append(perms, p)
			}
		}
	}
	if len(perms) == 0 {
		return ""
	}
	var parts []string
	for _, p := range perms {
		entry := fmt.Sprintf("`%s`", p.slug)
		if p.isTeam {
			entry += " *(team-scoped)*"
		}
		parts = append(parts, entry)
	}
	return fmt.Sprintf("**Required permissions:** %s", strings.Join(parts, ", "))
}

// Ensure errors package is used (for os.ErrNotExist in future use).
var _ = errors.New
