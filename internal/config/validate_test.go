package config

import "testing"

func TestServerFileValidateBasename(t *testing.T) {
	sf := &ServerFile{
		Servers: map[string]ServerEntry{
			"s1": {
				Replacements: map[string]FileOverrides{
					"config/app.properties": {"k": "v"},
				},
			},
		},
	}
	if err := sf.Validate(); err == nil {
		t.Fatal("expected path key error")
	}
}

func TestServerFileValidateOK(t *testing.T) {
	sf := &ServerFile{
		Servers: map[string]ServerEntry{
			"s1": {
				Replacements: map[string]FileOverrides{
					"game.properties": {"port": "1"},
				},
			},
		},
	}
	if err := sf.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(sf.ServerIDs()) != 1 {
		t.Fatal("server ids")
	}
}
