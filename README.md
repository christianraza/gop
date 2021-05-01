# gop
My personal Go packaging and release script.

## Dependencies
[Gox](https://github.com/mitchellh/gox)  
[Github CLI](https://github.com/cli/cli)

## Installation
```
$ go get github.com/christianraza/gop
```
## Usage
##### Package assets
```
$ gop -p
```
##### Release (without assets)
```
$ gop -r
```
##### Release (with assets)
```
$ gop -r -p
```
##### Pre-release (without assets)
```
$ gop --pre -r
```
##### Pre-release (with assets)
```
$ gop --pre -r -p
```
##### Help
```
$ gop -h
```

## Assumptions
gop assumes it will be run from the package root and that `main` is located there.  
gop assumes `go.mod` exists.  
gop assumes the beginning of the module path starts with a domain name such as `github.com/christianraza/gop`  
gop assumes the domain's protocol is `https` and will generate a readme accordingly.

## Notes
##### Configuration
To configure gop simply change the constants located near the top of `gop.go`
##### Changelog
gop expects a changelog to exist for versioning and release notes, by default it expects `CHANGELOG.md`.  
Changelog must look like:
```
# <version>
<notes>
```
The `#` followed by a space lets gop know that this is the version and below it are the notes for the release.  

When adding a new version for the next release simply prepend a `# <version>` and add notes below it.  

When gop looks for which version and what notes to use it will only use the most recent entry (the one at the top of the changelog).

##### Licenses and Notices
Licenses and notices are collected from the project root (if one exists there), and the `vendor` directory which gop will automatically generate if necessary.  
Supported license and notice names include LICENSE, COPYING, and NOTICE with any extension or capitalization.  
gop will generally gather all licenses that meet the requirements listed above but verify upon completion.
