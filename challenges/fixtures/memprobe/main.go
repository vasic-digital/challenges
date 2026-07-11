// memprobe — cross-process HelixMemory persistence prober for the
// persistent-memory Challenge. Subcommands:
//
//	write <db_path> <token>  : open the on-disk localstore, Add a memory
//	                           whose Content embeds <token>, then exit.
//	read  <db_path> <token>  : open the SAME on-disk file in a FRESH
//	                           process, Search for <token>, print RECALLED:<content>
//	                           on hit (exit 0) or MISS (exit 7).
//
// The two invocations are distinct OS processes sharing only the on-disk
// SQLite file — proving genuine cross-process persistence (CONST §11.4.5
// positive runtime evidence), not in-memory state carried in one process.
package main

import (
	"context"
	"fmt"
	"os"

	localstore "digital.vasic.helixmemory/pkg/localstore"
	modstore "digital.vasic.memory/pkg/store"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: memprobe <write|read> <db_path> <token>")
		os.Exit(2)
	}
	mode, dbPath, token := os.Args[1], os.Args[2], os.Args[3]
	ctx := context.Background()

	st, err := localstore.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open %s: %v\n", dbPath, err)
		os.Exit(3)
	}
	defer st.Close()

	switch mode {
	case "write":
		m := &modstore.Memory{
			Content: "persistent-memory-challenge fact token=" + token,
			Scope:   modstore.ScopeUser,
		}
		if err := st.Add(ctx, m); err != nil {
			fmt.Fprintf(os.Stderr, "add: %v\n", err)
			os.Exit(4)
		}
		cnt, _ := st.Count(ctx)
		fmt.Printf("WROTE id=%s count=%d db=%s\n", m.ID, cnt, st.Path())
	case "read":
		res, err := st.Search(ctx, token, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "search: %v\n", err)
			os.Exit(5)
		}
		for _, m := range res {
			if containsToken(m.Content, token) {
				fmt.Printf("RECALLED:%s\n", m.Content)
				os.Exit(0)
			}
		}
		fmt.Println("MISS")
		os.Exit(7)
	default:
		fmt.Fprintf(os.Stderr, "unknown mode %q\n", mode)
		os.Exit(2)
	}
}

func containsToken(s, tok string) bool {
	for i := 0; i+len(tok) <= len(s); i++ {
		if s[i:i+len(tok)] == tok {
			return true
		}
	}
	return false
}
