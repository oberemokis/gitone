#!/bin/sh
# Печатает теги сборки для -tags: какие грамматики tree-sitter уходят в
# бинарник для подсветки. Это набор core100 из gotreesitter без двух грамматик
# под копилефтом (caddy, disassembly) и vue, которого в core100 нет (#101,
# #139). Без тегов в бинарник уходят все двести с лишним, с GPL среди них.
# Список один на Makefile, CI и релиз: goreleaser берёт его из GITONE_TAGS.
set -eu

grammars="
bash c cpp c_sharp cmake css dart elixir elm erlang go gomod graphql
haskell hcl html ini java javascript json json5 julia kotlin lua make
markdown nix objc ocaml perl php powershell python r ruby rust scala scss
sql svelte swift toml tsx typescript xml yaml zig awk clojure d regex agda
embedded_template verilog ada angular apex arduino asm astro authzed bass
beancount bibtex bicep bitbake blade brightscript cairo capnp chatito
circom comment commonlisp cooklang corn cpon csv cuda cue cylc desktop
devicetree diff djot dockerfile doxygen dtd earthfile ebnf editorconfig eds
eex elsa enforce facility faust fennel
vue
"

tags="grammar_subset,gotreesitter_no_copyleft"
for grammar in $grammars; do
  tags="$tags,grammar_subset_$grammar"
done
echo "$tags"
