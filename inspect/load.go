package inspect

import (
	"fmt"
	"go/token"
	"path"
	"strings"

	"golang.org/x/tools/go/packages"
)

type LoadOptions struct {
	ProjectDir string
}

func LoadPackages(args []string, opts *LoadOptions) (*token.FileSet, []*packages.Package, error) {
	fset := token.NewFileSet()
	dir := ""
	if opts != nil {
		dir = opts.ProjectDir
	}
	cfg := &packages.Config{
		Dir:  dir,
		Mode: packages.NeedFiles | packages.NeedSyntax | packages.NeedDeps | packages.NeedImports | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedModule,
		Fset: fset,
		// BuildFlags: []string{"-a"}, // TODO: confirm what the extra non-gofile from
	}
	pkgs, err := packages.Load(cfg, args...)

	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			return nil, nil, fmt.Errorf("loading package error:%v %v", pkg, pkg.Errors)
		}
		normalizePackage(pkg)
	}
	return fset, pkgs, err
}

func MakePackageMap(pkgs []*packages.Package) map[string]*packages.Package {
	m := make(map[string]*packages.Package, len(pkgs))
	for _, pkg := range pkgs {
		m[normalizePkgPath(pkg)] = pkg
	}
	return m
}

func GetModuleDir(m *packages.Module) string {
	if m.Replace != nil {
		return m.Replace.Dir
	}
	return m.Dir
}

// GetSameModulePackagesAndPkgsGiven expand to all packages under
// the same module that depended by starter packages
func GetSameModulePackagesAndPkgsGiven(starterPackages []*packages.Package, wantsExtrPkgs map[string]bool, wantsExtrPkgsByMod map[string]bool) (sameModulePkgs []*packages.Package, extraPkgs []*packages.Package) {
	modules := make(map[string]bool)
	for _, pkg := range starterPackages {
		if pkg.Module != nil {
			modules[pkg.Module.Path] = true
		}
	}
	extraPkgs = make([]*packages.Package, 0, len(starterPackages))
	packages.Visit(starterPackages, func(p *packages.Package) bool {
		normed := false

		// filter test package of current module
		if p.Module != nil && modules[p.Module.Path] {
			normalizePackage(p)
			normed = true
			if !IsTestPkgOfModule(p.Module.Path, p.PkgPath) {
				// do not add test package to result, but still traverse its dependencies
				sameModulePkgs = append(sameModulePkgs, p)
			}
			return true
		}
		//  extra packages
		if p.Module != nil && len(wantsExtrPkgsByMod) > 0 && wantsExtrPkgsByMod[p.Module.Path] {
			if !normed {
				normalizePackage(p)
				normed = true
			}
			extraPkgs = append(extraPkgs, p)
			return true
		}
		if len(wantsExtrPkgs) > 0 {
			if !normed {
				normalizePackage(p)
				normed = true
			}
			if wantsExtrPkgs[p.PkgPath] {
				extraPkgs = append(extraPkgs, p)
				return true
			}
		}

		return true
	}, nil)
	return
}

func normalizePackage(pkg *packages.Package) {
	// normalize pkg path
	pkg.PkgPath = normalizePkgPath(pkg)
	pkg.Name = normalizePkgName(pkg)
}

func normalizePkgName(pkg *packages.Package) string {
	name := pkg.Name
	if name == "" {
		name = pkg.Types.Name()
	}
	return name
}

func normalizePkgPath(pkg *packages.Package) string {
	pkgPath := pkg.PkgPath
	if pkgPath == "" {
		pkgPath = pkg.Types.Path()
	}

	// normalize pkgPath
	if pkgPath == "command-line-arguments" {
		if len(pkg.GoFiles) > 0 {
			pkgPath = GetPkgPathOfFile(pkg.Module, path.Dir(pkg.GoFiles[0]))
		}
	}
	return pkgPath
}

// GetPath the returned result is guranteed to not end with "/"
// `filePath` is an absolute path on the filesystem.
func GetPkgPathOfFile(mod *packages.Module, fsPath string) string {
	modPath, modDir := mod.Path, mod.Dir
	if mod.Replace != nil {
		modPath, modDir = mod.Replace.Path, mod.Replace.Dir
	}

	rel, ok := RelPath(modDir, fsPath)
	if !ok {
		panic(fmt.Errorf("%s not child of %s", fsPath, modDir))
	}

	return strings.TrimSuffix(path.Join(modPath, rel), "/")
}

// RelPath returns "" if child is not in base
func RelPath(base string, child string) (sub string, ok bool) {
	if base == "" || child == "" {
		return "", false
	}
	last := base[len(base)-1]
	if last == '/' || last == '\\' { // last is separator
		base = base[:len(base)-1]
	}
	idx := strings.Index(child, base)
	if idx < 0 {
		return "", false // not found
	}
	// found base, check the case /a/b_c /a/b
	idx += len(base)
	if idx >= len(child) {
		return "", true
	}
	if child[idx] == '/' || child[idx] == '\\' {
		return child[idx+1:], true
	}
	return "", false
}

func GetFsPathOfPkg(mod *packages.Module, pkgPath string) string {
	modPath, modDir := mod.Path, mod.Dir
	if mod.Replace != nil {
		modPath, modDir = mod.Replace.Path, mod.Replace.Dir
	}
	if modPath == pkgPath {
		return modDir
	}
	if strings.HasPrefix(pkgPath, modPath) {
		return path.Join(modDir, pkgPath[len(modPath):])
	}
	panic(fmt.Errorf("%s not child of %s", pkgPath, modPath))
}
func GetRelativePath(modPath string, pkgPath string) string {
	if pkgPath == "" {
		panic(fmt.Errorf("GetRelativePath empty pkgPath"))
	}
	if strings.HasPrefix(pkgPath, modPath) {
		return pkgPath[len(modPath):]
	}
	panic(fmt.Errorf("%s not child of %s", pkgPath, modPath))
}
func GetFsPath(mod *packages.Module, relPath string) string {
	if mod.Replace != nil {
		return path.Join(mod.Replace.Dir, relPath)
	}
	return path.Join(mod.Dir, relPath)
}
