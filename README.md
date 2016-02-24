# SHIM

SHIM (also called *Shim*) is a web front-end interface for [Hugo](https://github.com/spf13/hugo).
Shim is meant to provide writers and editors with an easy way to draft, preview,
and publish content to their websites. Shim adds in the ease of use of dynamic
blogging platforms such as WordPress or Ghost to the speed Hugo, a static site
generation platform.

SHIM stands for Simple Hugo Interface and Manager.

## Why this Way?

The approach SHIM takes will lower the load of webhosts while still allowing the
users to dynamically create their content.

## Setup
!!! TODO: Finish this guide
!!! TODO: Explain config.toml

To set up Shim requires a few steps. Before attempting this guide, make sure that
Go is installed, and that you have a working `$GOBIN`, `$GOPATH`, and `$GOROOT`.

The first step is installing Hugo. After this, an environment must be set up so
that shim can actually do its job.

```
$ go get github.com/spf13/hugo
$ git clone https://github.com/camconn/shim
$ cd shim
$ go build
$ ./shim
```

After all this, Shim will have created a test site, `test` for you in the `sites/` folder.

## Contributing
If you wish to contribute to Shim, you must assign the project maintainer the copyright
of any non-significant contribution. Feel free to submit a pull request.

## License
Shim is released under the GNU Affero General Public License, Version 3 or any
later version.  The source code for this program is available on
[Github](https://github.com/camconn/shim).
