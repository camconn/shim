# SHIM

**SHIM IS ALPHA QUALITY™ SOFTWARE. USE CAUTION AS WELL AS BACKUPS!!**

```
     _     _
 ___| |__ (_)_ __ ___
/ __| '_ \| | '_ ` _ \
\__ \ | | | | | | | | |
|___/_| |_|_|_| |_| |_|

shim the hugo interface and manager
```

SHIM (also called *Shim* or *shim*) is a blog-focused interface for
[Hugo](https://github.com/spf13/hugo).

Shim provides the ease-of-use of traditional blog engines like Wordpress with
the blistering speed of static site generators. Because Shim lets Hugo do all
of the work associated with site generation, Shim can focus exclusively on
providing the best user experience.

## Why?

One traditional downside of static site generators is that the writer cannot see
how their content will look until after they run a generation command.

Shim solves this by handling all interaction with the with Hugo, including
changing site settings, saving posts, and generating preview and public builds
automatically for the user. This allows content creators to focus on what they
do best: producing and uploading content.

## Setup
It takes a few steps to set up Shim. Before attempting this guide, make sure that
Go 1.5 or later is installed, and that you have a working `$GOBIN`, `$GOPATH`,
and `$GOROOT`.

The first step is installing Hugo. After that, Shim's dependencies must be satisfied.

```
$ go get github.com/spf13/hugo
$ git clone https://github.com/camconn/shim
$ cd shim
$ go get -v github.com/BurntSushi/toml \
            github.com/justinas/alice \
            github.com/niemal/uman \
            github.com/spf13/viper
$ git submodule init  # fetches default themes
$ go build
$ ./shim
# At this point, a message will printed out telling you the default credentials
# for your shim instance.
```

After all this, Shim will have created a test site, `test` for you in the `sites/` folder.

By default, Shim opens an http server at 127.0.0.1 on port 8080, however it will
use a different port if the `PORT` env variable is set.

To run Shim on top of a dynamic reload utility like [gin](https://github.com/codegangsta/gin),
you can run the following command:
```
$ gin -p 8080 run
```

Shim-specific settings may be changed by modifying `config.toml`.

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
