# inpxer

OPDS 1.1 and web server for `.inpx` libraries with full-text search.

## Fork changes

This is a fork of [shemanaev/inpxer](https://github.com/shemanaev/inpxer). Differences from upstream:

- Book archives can be `.7z` as well as `.zip`; the format is detected from the file signature.
  If the inpx doesn't name the archive, `<inp name>.zip` is tried first, then `<inp name>.7z`.
- Built with Go 1.27 (pinned in `.tool-versions`).
- Pushing a `v*` tag publishes a GitHub release and `linux/amd64` images to
  [`ghcr.io/stanvit/inpxer`](https://github.com/stanvit/inpxer/pkgs/container/inpxer) in two flavours (see [Docker](#docker)).

Reading from solid 7z archives means decompressing everything before the book in its solid block,
so downloads from large solid blocks can be slow.

## Usage

### Standalone

Download the latest release.
Download [`inpxer-example.toml`](./inpxer-example.toml), rename to `inpxer.toml`, put next to executable (or current working directory) and edit to your liking.

Import data:
```shell
./inpxer import ./file.inpx
```

You can specify `--partial` flag to import only new records and keep old ones.
Otherwise, old index data will be deleted (the whole folder specified in `index_path`) and reindex from scratch.

Start server:
```shell
./inpxer serve
```

Web interface will be available on [http://localhost:8080/](http://localhost:8080/) and
OPDS will be on [http://localhost:8080/opds](http://localhost:8080/opds) by default.

### Docker

Images are published to [`ghcr.io/stanvit/inpxer`](https://github.com/stanvit/inpxer/pkgs/container/inpxer) in two flavours:

| Flavour | Tags | Contents |
|---|---|---|
| plain | `latest`, `0.4`, `0.4.2` | inpxer only |
| fbc | `latest-fbc`, `0.4-fbc`, `0.4.2-fbc` | inpxer and the [fb2cng](https://github.com/rupor-github/fb2cng) converter at `/bin/fbc` ([GPL-3.0](https://github.com/rupor-github/fb2cng/blob/main/LICENSE)) |

To offer fb2 to epub conversion with the fbc flavour, add to `inpxer.toml`:
```toml
[[converters]]
from = "fb2"
to = "epub"
command = "/bin/fbc"
arguments = "convert --to epub2 -o {to} {from}"
```

Locally, `docker build --target fbc .` builds the fbc flavour; the default target is plain.

Download [`inpxer-example.toml`](./inpxer-example.toml), rename to `inpxer.toml` and edit to your liking.

inpxer expects config file to be at `/data/inpxer.toml`.

Import data:
```shell
docker run --rm -it -v ${PWD}:/import -v <path to data storage>:/data ghcr.io/stanvit/inpxer inpxer import /import/file.inpx
```

*Note: existing index will be deleted.*

Start server:
```shell
docker run -it -p 8080:8080 -v <path to data storage>:/data ghcr.io/stanvit/inpxer
```
