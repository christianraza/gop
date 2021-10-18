package main

import (
	"archive/zip"
	"bufio"
	"flag"
	"fmt"
	"go/build"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Configuration
const (
	// Distributions directory
	distDir = "dist"
	// Binaries directory
	binDir = "bin"
	// Changelog name
	logName = "CHANGELOG.md"
	// Packaged license directory
	packLicDir = "licenses-and-notices"
	// Packaged readme name
	packReadmeName = "readme.txt"
	// Module domain protocol
	protocol = "https://"
)

// Files to ignore when traversing the walk directory
var noWalk = map[string]struct{}{
	"examples": {},
	"assets":   {},
	"dist":     {},
	"bin":      {},
	"src":      {},
	".go":      {},
}

var version string
var releaseFlag bool
var changelog string
var packFlag bool
var prerelease bool
var projectName string
var modulePath string

var logErr *log.Logger = log.New(os.Stdout, "", log.Lshortfile)

type copyFunc func(root string, path string, info fs.FileInfo) io.Writer

func main() {
	flag.BoolVar(&releaseFlag, "r", false, "Release to Github")
	flag.BoolVar(&packFlag, "p", false, "Package")
	flag.BoolVar(&prerelease, "pre", false, "Mark as pre-release")
	flag.Parse()

	// Get project info
	projectInfo("go.mod")

	// Get version and changelog
	changes(logName)

	// Package binaries
	if packFlag {
		pack()
	}

	// Release
	if releaseFlag {
		release(distDir)
	}
}

func projectInfo(s string) {
	f, err := os.Open(s)
	if err != nil {
		logErr.Fatal("Please generate mod file, use: go mod init <path>")
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if fields[0] == "module" {
			modulePath = fields[1]
			a := strings.Split(fields[1], "/")
			projectName = a[len(a)-1]
			break
		}
	}
	if err := scanner.Err(); err != nil {
		logErr.Fatal(err)
	}
}

func changes(s string) {
	f, err := os.Open(s)
	if err != nil {
		logErr.Fatalf("Please add %s\n", logName)
	}
	defer f.Close()
	var b strings.Builder
	in := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		t := scanner.Text()
		if len(t) > 2 && t[0:2] == "# " {
			in = !in
			if !in {
				break
			}
			version = strings.TrimSpace(t[1:])
		} else {
			b.WriteString(t)
			b.WriteString("\n")
		}
	}

	if err := scanner.Err(); err != nil {
		logErr.Fatal(err)
	}

	changelog = strings.TrimSuffix(b.String(), "\n")
}

func pack() {
	// Make directories if !exist else truncate
	mkdirOrTruncate(distDir)
	mkdirOrTruncate(binDir)

	// Run gox
	runGox(binDir)

	// Get binaries
	binaries, err := ioutil.ReadDir(binDir)
	if err != nil {
		logErr.Fatal(err)
	}

	// Get vendors
	err = exec.Command("go", "mod", "vendor").Run()
	if err != nil {
		logErr.Fatal(err)
	}

	// Collect licenses
	tempDir, err := ioutil.TempDir(".", "temp")
	if err != nil {
		logErr.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	collect("vendor", tempDir)
	// Collect project license
	files, err := ioutil.ReadDir(".")
	if err != nil {
		logErr.Fatal(err)
	}
	found := false
	for _, file := range files {
		if isLicense(file.Name()) {
			f, err := os.Create(filepath.Join(tempDir, projectName+"-"+strings.ToLower(file.Name())))
			if err != nil {
				logErr.Fatal(err)
			}
			copy(file.Name(), f)
			f.Close()
			found = true
			break
		}
	}
	if !found {
		fmt.Fprintf(os.Stderr, "\n\u2757 Packaging %s without license\n", projectName)
	}

	// Readme
	readme := readme(projectName)

	// Package binaries
	fmt.Printf("\nPackaging:\n\n")
	for _, bin := range binaries {
		ext := filepath.Ext(bin.Name())
		base := strings.TrimSuffix(bin.Name(), ext)
		// Create zip
		fmt.Printf("\U0001F4E6 %s\n", base+".zip")
		f, err := os.Create(filepath.Join(distDir, base+".zip"))
		if err != nil {
			logErr.Fatal(err)
		}
		defer f.Close()
		w := zip.NewWriter(f)
		defer w.Close()
		// Write binary to zip
		to, err := w.Create(projectName + ext)
		if err != nil {
			logErr.Fatal(err)
		}
		copy(filepath.Join(binDir, bin.Name()), to)
		// Write readme to zip
		to, err = w.Create(packReadmeName)
		if err != nil {
			logErr.Fatal(err)
		}
		_, err = io.Copy(to, strings.NewReader(readme))
		if err != nil {
			logErr.Fatal(err)
		}
		// Write relDir to zip
		copyWalk(tempDir, func(root string, path string, info fs.FileInfo) io.Writer {
			to, err := w.Create(filepath.Join(packLicDir, info.Name()))
			if err != nil {
				logErr.Fatal(err)
			}
			return to
		})
	}
}

func isLicense(fname string) bool {
	name := strings.ToLower(strings.TrimSuffix(fname, filepath.Ext(fname)))
	return (name == "license" || name == "copying" || name == "notice")
}

func mkdirOrTruncate(name string) {
	if _, err := os.Stat(name); !os.IsNotExist(err) {
		os.RemoveAll(name)
	}
	err := os.Mkdir(name, os.ModeDir)
	if err != nil {
		logErr.Fatal(err)
	}
}

func exists(bin []fs.FileInfo, s string) bool {
	for _, b := range bin {
		if b.Name() == s {
			return true
		}
	}
	return false
}

func runGox(dir string) {
	// Check if gox exists in GOBIN
	path := filepath.Join(build.Default.GOPATH, "bin")
	bin, err := ioutil.ReadDir(path)
	if err != nil {
		logErr.Fatal(err)
	}
	var exist bool
	if runtime.GOOS == "windows" {
		exist = exists(bin, "gox.exe")
	} else {
		exist = exists(bin, "gox")
	}
	if !exist {
		logErr.Fatal("Please install gox before packaging, use: go get github.com/mitchellh/gox")
	}
	// Execute gox
	var cmd *exec.Cmd
	flags := []string{
		"-output=\"" + filepath.Join(dir, "{{.Dir}}-{{.OS}}-{{.Arch}}") + "\"",
	}
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell", "gox "+strings.Join(flags, " "))
	} else {
		cmd = exec.Command("gox", strings.Join(flags, " "))
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout
	if err := cmd.Run(); err != nil {
		fmt.Printf("gox errors ^\n")
	}
}

func runCmd(cmd *exec.Cmd) (err error) {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout
	err = cmd.Run()
	if err != nil {
		return
	}
	return
}

func collect(vend, dir string) {
	if _, err := os.Stat(vend); !os.IsNotExist(err) {
		copyWalk(vend, func(root string, path string, info fs.FileInfo) io.Writer {
			name := strings.ToLower(info.Name())
			if isLicense(name) {
				// Parent
				pPath := filepath.Dir(path)
				pName := filepath.Base(pPath)
				// Grand parent
				gpPath := filepath.Dir(pPath)
				gpName := filepath.Base(gpPath)
				// Desired base name
				base := strings.Join([]string{gpName, pName, name}, "-")
				// Create file
				to, err := os.Create(filepath.Join(dir, base))
				if err != nil {
					logErr.Fatal(err)
				}
				return to
			}
			return nil
		})
	}
}

func copyWalk(dir string, f copyFunc) {
	err := filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if err != nil {
			return err
		}
		// Skip paths
		_, a := noWalk[info.Name()]
		_, b := noWalk[filepath.Ext(info.Name())]
		if a || b {
			if info.IsDir() {
				return filepath.SkipDir
			} else {
				return nil
			}
		}
		if to := f(dir, path, info); to != nil {
			copy(path, to)
		}
		return nil
	})
	if err != nil {
		logErr.Fatal(err)
	}
}

func copy(from string, to io.Writer) (err error) {
	var f *os.File
	f, err = os.Open(from)
	if err != nil {
		return
	}
	defer f.Close()
	_, err = io.Copy(to, f)
	if err != nil {
		return
	}
	return
}

func readme(name string) string {
	var b strings.Builder
	b.WriteString("Thank you for downloading ")
	b.WriteString(strings.Title(name))
	b.WriteString("\n")
	b.WriteString("If you would like to contribute and/or download the source code, visit:\n")
	b.WriteString(protocol)
	b.WriteString(modulePath)
	b.WriteString("\n")
	return b.String()
}

func release(dir string) {
	assets, err := ioutil.ReadDir(dir)
	if err != nil {
		logErr.Fatal(err)
	}
	if packFlag && len(assets) <= 0 {
		fmt.Fprintf(os.Stderr, "No assets in %s directory\n", dir)
		os.Exit(0)
	}

	fmt.Printf("\nReleasing:\n\n")
	// Write changelog to temporary file
	tmp, err := ioutil.TempFile(".", "temp*.md")
	if err != nil {
		logErr.Fatal(err)
	}
	_, err = io.Copy(tmp, strings.NewReader(changelog))
	if err != nil {
		logErr.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	// Create release
	fmt.Printf("\U0001F3F7 %s\n", version)
	args := []string{"release", "create", version, "-t", version, "-F", tmp.Name()}
	if prerelease {
		args = append(args, "-p")
	}
	cmd := exec.Command("gh", args...)
	err = runCmd(cmd)
	if err != nil {
		// Cleanup on errors
		tmp.Close()
		os.Remove(tmp.Name())
		logErr.Fatal(err)
	}
	tmp.Close()

	if packFlag {
		fmt.Printf("\nUploading~\n\n")
		args := []string{"release", "upload", version}
		for _, a := range assets {
			fmt.Printf("\U0001F4EC %s\n", a.Name())
			args = append(args, filepath.Join(dir, a.Name()))
		}

		cmd = exec.Command("gh", args...)
		err = runCmd(cmd)
		if err != nil {
			logErr.Fatal(err)
		}
	}
}
