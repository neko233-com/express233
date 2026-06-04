package api

import "github.com/neko233-com/express233/internal/store"

func mustServerYAML(st *store.Store) string {
	p, err := st.ServerYAMLPath(1)
	if err != nil {
		panic(err)
	}
	return p
}
