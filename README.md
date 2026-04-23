## WIP CLI Tool for SC that I am experimenting with:

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
