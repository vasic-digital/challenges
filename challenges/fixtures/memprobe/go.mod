// Isolated module for the persistent-memory Challenge cross-process prober.
// Kept OUT of the main digital.vasic.challenges module so the heavy
// modernc.org/sqlite dependency tree does not pollute it. Replace directives
// are consumer-side build overrides only (CONST-051(B/C)); memprobe imports
// only the public localstore + store APIs — no source-level coupling.
module digital.vasic.challenges.memprobe

go 1.25.0

require (
	digital.vasic.helixmemory v0.0.0-00010101000000-000000000000
	digital.vasic.memory v0.0.0-00010101000000-000000000000
)

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ncruces/go-strftime v0.1.9 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/exp v0.0.0-20250408133849-7e4ce0ab07d0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	modernc.org/libc v1.65.7 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
	modernc.org/sqlite v1.37.1 // indirect
)

replace digital.vasic.helixmemory => ../../../../helix_memory

replace digital.vasic.memory => ../../../../memory
