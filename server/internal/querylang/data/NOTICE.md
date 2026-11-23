# Local language identification resources

ZimaOS Blue reuses ZimaOS Photos' language detector and includes the unmodified
`lid.176.ftz` model by Facebook Research /
fastText, licensed under Creative Commons Attribution-ShareAlike 3.0 Unported.
See the accompanying `CC-BY-SA-3.0.txt` for the full license.

Source and attribution: https://fasttext.cc/docs/en/language-identification.html
Model: https://dl.fbaipublicfiles.com/fasttext/supervised-models/lid.176.ftz
SHA256: 8f3472cfe8738a7b6099e8e999c3cbfae0dcd15696aac7d7738a8039db603e83
Model bytes: 938013. Model modifications: none.

The pure Go inference adaptation follows fastText v0.9.2, commit
5b5943c118b0ec5fb9cd8d20587de2b2d3966dfe.
Copyright (c) 2016-present, Facebook, Inc. MIT license; see `LICENSE.fasttext`.

The model identifies languages locally. Blue uses its configured LLM for optional
skill-query rewriting; no Photos NMT runtime or language packs are included.

## OpenCC derived character evidence

`simplified-zh-evidence.txt` is a modified, derived subset of OpenCC dictionary
data by BYVoid and OpenCC contributors, licensed under Apache License 2.0.
The full upstream license is included as `LICENSE.OpenCC`.

Source: https://github.com/BYVoid/OpenCC
Revision: fae2a0a911c30764e669a7d8b0591cd8e852934a
Inputs: `data/dictionary/STCharacters.txt` and `JPShinjitaiCharacters.txt`.
Modifications by ZimaOS Photos: retain single-character simplified keys with no
self mapping, exclude all Japanese shinjitai keys, deduplicate and sort.
The resulting 3,689 characters are used as a heuristic for query language
identification; this is not an exhaustive inventory of Japanese kanji.

Derived asset SHA256:
7110890e19cdef71f78b558ced2694bdd5ff792b4f2c76885d8b75e2f96fd49e
