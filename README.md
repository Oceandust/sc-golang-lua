## WIP CLI Tool for SC that I am experimenting with:


### Build
Non-lua:

```bash 
go build -o sc_cli .
```

With LuaJIT (requires [LuaJIT v2.1.0-beta1](https://github.com/LuaJIT/LuaJIT/tree/v2.1.0-beta1))
```bash 
LJ=/somewhere/luajit-2.1-beta1
CGO_CFLAGS="-I$LJ/include/luajit-2.1" \
CGO_LDFLAGS="-L$LJ/lib -Wl,-rpath,$LJ/lib" \
go build -tags luajit -o sc_cli .
```
### CLI commands

#### `tpak-unpack`

```bash
./sc_cli tpak-unpack [--threads 4] [--dump-metadata] <input-dir> <output-dir>
```

Unpacks every `.pak` file directly inside `<input-dir>`.
Each archive is extracted into `<output-dir>\<archive-name>`.
`--threads` controls concurrent file extraction workers per archive and defaults to `4`.
Use `--dump-metadata` to also write metadata sidecar files for byte-identical repacking workflows (TODO, not sure if they will be needed in the future).

Note that path traversal was not in mind when making this, so don't run this on arbitrary data.

#### `tpak-unpack-file`

```bash
./sc_cli tpak-unpack-file [--threads 4] [--dump-metadata] <input-pak> <output-dir>
```

Unpacks one `.pak` file directly into `<output-dir>`.
Unlike `tpak-unpack`, this does not create an extra `<archive-name>` directory under the output directory.
`--threads` controls concurrent file extraction workers and defaults to `4`.
Use `--dump-metadata` to also write metadata sidecar files into `<output-dir>`.

#### `tpak-inspect`

```bash
./sc_cli tpak-inspect [--format text|json] <input-pak>
```

Prints the parsed internal structure of one `.pak` file without extracting payloads.
The output includes the header, table sizes, name records, file index, file entries, files, and chunks in archive order.
`--format` defaults to `text`.

#### `tpak-pack` (not fully functional)

```bash
./sc_cli tpak-pack <input-dir> <output-dir>
```

Packs every top-level directory inside `<input-dir>` into `<output-dir>\<directory-name>.pak`.
If an archive directory contains `.tpak.meta.json` and `.tpak.raw`, the packer uses that metadata when it can preserve original ordering and chunk payloads.
Without metadata, it builds a fresh archive from the files in the directory.

#### `yup-rewrite` (experimental)

```bash
./sc_cli yup-rewrite [--out <output-file>] <manifest> <root-dir>
```

Rewrites a SC `.yup` manifest using file metadata from `<root-dir>`.
If `--out` is omitted, the input manifest is overwritten.

#### `snapshot export` (experimental)

```bash
./sc_cli snapshot export --compiled-root <compiled-root> [--root <repo-root>] [--out <output-file>]
```

Loads the compiled game data (needs to be unpacked first) and writes a normalized snapshot JSON file.
`--root` defaults to `..`.
`--out` defaults to `out/sc_cli.snapshot.json`.

#### `defs inspect` (experimental)

```bash
./sc_cli defs inspect --compiled-root <compiled-root> --id <definition-id> [--root <repo-root>] [--format text|json]
```

Loads the snapshot data and prints the normalized records matching `<definition-id>`.
`--format` defaults to `text`.
`--root` defaults to `..`.

#### `defs check` (experimental)

```bash
./sc_cli defs check --compiled-root <compiled-root> [--root <repo-root>]
```

Loads the snapshot data and runs the built-in validation checks for a few def names.
`--root` defaults to `..`.

### tpak packer/unpacker (inspired by and credit to [this project](https://github.com/Johnnynator/tpak))
unpacker is straightforward, could be parallelized to speed things up or made more configurable

packer is a bit tricky as you can see by the dumped metadata by the unpacker.

integrity is verified by some file metadata and hashes inside the .yup files in the root directory, so thats what the yup (the yup format is very torrent-y) rewriter is for.

it will naively re-index and re-calculate everything (not ideal, but works for now)

### Def Graph dumper/transformer/...
Anyone familiar with the internal structure knows it's a mess to work with.

This was just an experiment on how to interact with the LuaJIT engine. Guarded by buildtag and requires LuaJIT [v2.1.0-beta1](https://github.com/LuaJIT/LuaJIT/tree/v2.1.0-beta1)

The goal is to have this tool output normalized data to work with in the future. 

The current JSON output is too inefficient (over 1 GB of data for the referenced files), will probably add sqllite output later or something else.

Types are incomplete and a mess, verification command is just a quick way to test some data is actually present.

So a big TODO
