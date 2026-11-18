package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestHistoryAppends(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	for _, in := range []string{"gia sou", "ti kaneis?"} {
		h, err := openHistory()
		if err != nil {
			t.Fatal(err)
		}
		if err := h.add(in, "out:"+in); err != nil {
			t.Fatal(err)
		}
		h.Close()
	}
	path, _ := historyPath()
	if filepath.Base(filepath.Dir(path)) != "greeklish" {
		t.Fatalf("unexpected path %s", path)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	var got []entry
	for dec.More() {
		var e entry
		if err := dec.Decode(&e); err != nil {
			t.Fatal(err)
		}
		got = append(got, e)
	}
	if len(got) != 2 || got[1].Input != "ti kaneis?" || got[1].Output != "out:ti kaneis?" || got[0].Time.IsZero() {
		t.Fatalf("history entries: %+v", got)
	}
}
