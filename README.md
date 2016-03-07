# SHIM

**SHIM IS ALPHA QUALITY SOFTWARE. USE CAUTION AS WELL AS BACKUPS!!**

SHIM (also called *Shim*) is a web front-end interface for [Hugo](https://github.com/spf13/hugo).
Shim is meant to provide writers and editors with an easy way to draft, preview,
and publish content to their websites. Shim adds in the ease of use of dynamic
blogging platforms such as WordPress or Ghost to the speed Hugo, a static site
generation platform.

SHIM stands for Simple Hugo Interface and Manager.

## Why this Way?

One traditional downside of static site generators is that the writer cannot see
how their content will look until after they run a generation command. Shim intends
to wedge (or *shim*) itself between the content producers and the site generator.

The approach SHIM takes will lower the load of web hosts while still allowing the
users to dynamically create their content (similar to Ghost).

## Setup
To set up Shim requires a few steps. Before attempting this guide, make sure that
Go is installed, and that you have a working `$GOBIN`, `$GOPATH`, and `$GOROOT`.

The first step is installing Hugo. After this, an environment must be set up so
that shim can actually do its job.

```
$ go get github.com/spf13/hugo
$ git clone https://github.com/camconn/shim
$ cd shim
$ go get -v github.com/BurntSushi/toml \
            github.com/justinas/alice \
            github.com/niemal/uman \
            github.com/spf13/viper
$ git submodule init
$ go build
$ ./shim
```

After all this, Shim will have created a test site, `test` for you in the `sites/` folder.

By default, Shim opens an http server at 127.0.0.1 on port 8080, however it will
use a different port if the `PORT` env variable is set.

To run Shim on top of a dynamic reload utility like [gin](https://github.com/codegangsta/gin),
you can run the following command:
```
$ gin -p 8080 run
```

For further configuration, you can modify `config.toml`. If that file doesn't
exist, then copy `config.toml.example` to `config.toml` and edit the newly
copied file.

## Contributing
If you wish to contribute to Shim, you **must** grant the project maintainer
(Cameron Conn) a right to copy, modify, redistribute, and relicense your
contribution(s). All contributions via normal means (including but not limited
to Github pull requests and patches) are assumed to be under this condition.
If a contribution does not meet this litmus test, it will be denied.

Moreover, to improve the code-quality of Shim, please run `go vet`, `go lint`,
and `go fmt` on your code before submitting a pull request. This ensures that
the project's code looks consistent and readable. Don't be surprised if your
pull request is rejected if it does not meet these guidelines.

For now, there is no code of conduct for contributing to Shim. This may change
in the future. For now, just use common sense and act mature whenever on interacting
with others via official project channels.

## License
Shim is released under the GNU Affero General Public License, Version 3 or any
later version. A copy of this license can be found in the file `LICENSE`.

The source code for this program is available on [Github](https://github.com/camconn/shim).
