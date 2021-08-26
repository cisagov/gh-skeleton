# gh-skeleton #

[![GitHub Build Status](https://github.com/cisagov/gh-skeleton/workflows/build/badge.svg)](https://github.com/cisagov/gh-skeleton/actions)

This extension for the [`gh` CLI] provides the
ability to clone repositories from our existing library of skeleton
repositories.

## Prerequisites ##

A working installation of the [`gh` CLI].

## Installation ##

To install the extension issue the following command:

```console
gh extension install cisagov/gh-skeleton
```

## Usage ##

To list the available skeletons:

```console
gh skeleton list
```

To create a new local repository from a skeleton:

```console
gh skeleton clone skeleton-generic my-repo
```

Additional help is available from the extension:

```console
gh skeleton --help
```

## Updating ##

To update the extension issue the following command:

```console
gh extension upgrade skeleton
```

## Uninstalling ##

To uninstall the extension issue the following command:

```console
gh extension remove skeleton
```

## Contributing ##

We welcome contributions!  Please see [`CONTRIBUTING.md`](CONTRIBUTING.md) for
details.

## License ##

This project is in the worldwide [public domain](LICENSE).

This project is in the public domain within the United States, and
copyright and related rights in the work worldwide are waived through
the [CC0 1.0 Universal public domain
dedication](https://creativecommons.org/publicdomain/zero/1.0/).

All contributions to this project will be released under the CC0
dedication. By submitting a pull request, you are agreeing to comply
with this waiver of copyright interest.

[`gh` CLI]: https://github.com/cli/cli
