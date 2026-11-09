package notify

import "testing"

func TestSendPassesTitleAndBody(t *testing.T) {
	var got []string
	n := &Notifier{path: "/usr/bin/notify-send", run: func(name string, args ...string) error {
		got = append([]string{name}, args...)
		return nil
	}}
	n.once.Do(func() {}) // keep the fake path
	if err := n.Send("Apogée", "Une vague approche"); err != nil {
		t.Fatal(err)
	}
	if len(got) < 3 || got[len(got)-2] != "Apogée" || got[len(got)-1] != "Une vague approche" {
		t.Fatalf("command %q", got)
	}
}

func TestSendWithoutNotifierIsSilent(t *testing.T) {
	n := &Notifier{run: func(string, ...string) error { t.Fatal("ran a missing notifier"); return nil }}
	n.once.Do(func() {}) // nothing found
	if err := n.Send("a", "b"); err != nil {
		t.Fatal(err)
	}
}
