# Third-party notices

| Field | Value |
|---|---|
| Repository | `digital.vasic.challenges` |
| Revision | measured 2026-11-24 against the `go.mod` files named below |
| Referenced by | `LICENSE` section 4 |
| Gate | `scripts/verify_licence.sh` requires this file |

`LICENSE` section 4 states that third-party components remain under their own licences,
listed in this file. This file lists them. It does not grant, change or restrict any right
under those licences.

## Scope and method

- **Nothing is vendored.** This repository contains no copied third-party source tree
  (`vendor/`, `third_party/`, `node_modules/`) and no git submodules. Third-party Go modules
  are resolved at build time from `go.mod` / `go.sum`. Every file in the repository that
  carries a copyright line names the repository owner.
- **Source of the list:** the `require` blocks of the `go.mod` named below, direct and
  indirect. Own-organisation modules (`digital.vasic.*`) are not third-party and are listed
  separately.
- **How each licence was determined:** the module at the pinned version was downloaded with
  `go mod download` and its licence file read; the licence name was assigned from the licence
  text. A row says UNKNOWN where no licence file could be read.
- **Not covered:** the Go standard library and toolchain (distributed under the Go
  project's BSD-3-Clause licence, not copied into this repository) and tools a developer may
  install separately.

## Go modules required by `go.mod`

| Module | Version | Relation | Licence | Licence file |
|---|---|---|---|---|
| `github.com/gorilla/websocket` | `v1.5.3` | direct | BSD-2-Clause | LICENSE |
| `github.com/stretchr/testify` | `v1.11.1` | direct | MIT | LICENSE |
| `gopkg.in/yaml.v3` | `v3.0.1` | direct | Apache-2.0 | LICENSE +NOTICE |
| `github.com/davecgh/go-spew` | `v1.1.2-0.20180830191138-d8f796af33cc` | indirect | ISC | LICENSE |
| `github.com/kr/pretty` | `v0.3.1` | indirect | MIT | License (read manually) |
| `github.com/pmezard/go-difflib` | `v1.0.1-0.20181226105442-5d4384ee4fb2` | indirect | BSD-2-Clause | LICENSE |
| `github.com/stretchr/objx` | `v0.5.3` | indirect | MIT | LICENSE |
| `gopkg.in/check.v1` | `v1.0.0-20201130134442-10cb98267c6c` | indirect | BSD-2-Clause | LICENSE |

## Go modules required by `challenges/fixtures/memprobe/go.mod`

| Module | Version | Relation | Licence | Licence file |
|---|---|---|---|---|
| `github.com/dustin/go-humanize` | `v1.0.1` | indirect | MIT | LICENSE |
| `github.com/google/uuid` | `v1.6.0` | indirect | BSD-3-Clause | LICENSE |
| `github.com/mattn/go-isatty` | `v0.0.20` | indirect | MIT | LICENSE |
| `github.com/ncruces/go-strftime` | `v0.1.9` | indirect | MIT | LICENSE |
| `github.com/remyoudompheng/bigfft` | `v0.0.0-20230129092748-24d4a6f8daec` | indirect | BSD-3-Clause | LICENSE |
| `golang.org/x/exp` | `v0.0.0-20250408133849-7e4ce0ab07d0` | indirect | BSD-3-Clause | LICENSE |
| `golang.org/x/sys` | `v0.33.0` | indirect | BSD-3-Clause | LICENSE |
| `modernc.org/libc` | `v1.65.7` | indirect | BSD-3-Clause | LICENSE |
| `modernc.org/mathutil` | `v1.7.1` | indirect | BSD-3-Clause | LICENSE |
| `modernc.org/memory` | `v1.11.0` | indirect | BSD-3-Clause | LICENSE |
| `modernc.org/sqlite` | `v1.37.1` | indirect | BSD-3-Clause | LICENSE |

## Own-organisation modules (not third-party)

| Module | Where | Note |
|---|---|---|
| `digital.vasic.containers` | `replace => ../containers` in `go.mod` | Sibling repository of the same owner, under its own source-available licence. Not copied here. |
| `digital.vasic.helixmemory`, `digital.vasic.memory` | `replace` targets in `challenges/fixtures/memprobe/go.mod` | Own-organisation modules outside this repository; their licence was not examined from this repository. UNKNOWN here. |

## Items that could not be verified

No third-party module above is UNKNOWN; the licence of the two own-organisation memprobe replace targets is UNKNOWN from this repository (see above). Licence names were assigned from licence text
and not from legal review; consult each module's own licence file for the binding terms.
