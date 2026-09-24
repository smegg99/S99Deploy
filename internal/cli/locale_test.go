// internal/cli/locale_test.go

package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/smegg99/s99logger"

	"github.com/smegg99/s99deploy/internal/deploy/deploytest"
	"github.com/smegg99/s99deploy/internal/messages"
)

func TestLangFromArgs(t *testing.T) {
	for _, c := range []struct {
		name string
		args []string
		want string
		ok   bool
	}{
		{name: "absent", args: []string{"up", "myapp"}},
		{name: "joined", args: []string{"--lang=pl", "--help"}, want: "pl", ok: true},
		{name: "separate", args: []string{"--lang", "pl", "up", "myapp"}, want: "pl", ok: true},
		{name: "after the command", args: []string{"up", "--lang", "pl", "myapp"}, want: "pl", ok: true},
		{name: "the last one wins", args: []string{"--lang=pl", "--lang=en"}, want: "en", ok: true},
		{name: "after the terminator", args: []string{"--", "--lang=pl"}},
		{name: "with no value", args: []string{"up", "--lang"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, ok := langFromArgs(c.args)
			if got != c.want || ok != c.ok {
				t.Errorf("langFromArgs(%v) = %q, %v, want %q, %v", c.args, got, ok, c.want, c.ok)
			}
		})
	}
}

// The language has to be known before the tree exists.
func TestLangReachesHelpAndFlagDescriptions(t *testing.T) {
	// Short, Long and every flag description are set at construction.
	opts, stdout, _ := testOptions(t, "")

	if code := run(context.Background(), opts, []string{"--lang=pl", "--help"}); code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}

	help := stdout.String()
	for _, want := range []string{"Dostępne polecenia:", "Wdrożenie:", "język, w którym mówi to polecenie"} {
		if !strings.Contains(help, want) {
			t.Errorf("help is not in Polish, missing %q:\n%s", want, help)
		}
	}
}

func TestUnsupportedLangIsAUsageError(t *testing.T) {
	opts, _, stderr := testOptions(t, "")

	code := run(context.Background(), opts, []string{"--lang=de", "up", "myapp"})

	if code != 2 {
		t.Errorf("code = %d, want 2", code)
	}
	for _, want := range []string{"--lang de", "en", "pl", "Usage:"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr is missing %q:\n%s", want, stderr)
		}
	}
}

// The log stream is the operator's too, not only the help text.
func TestLoggerRendersSentencesAndNotIds(t *testing.T) {
	// Without a Translator, s99logger writes the message id, so an operator
	// reads "logs.installed" where a sentence belongs.
	sink := &deploytest.Sink{}
	log := s99logger.New(sink, s99logger.Options{
		Service: "s99deploy", Language: "pl", Translator: messages.Translator(),
	})

	log.Info(s99logger.NewEvent(messages.KeyLogsInstalled))

	if len(sink.Records) != 1 {
		t.Fatalf("got %d records, want 1", len(sink.Records))
	}
	record := sink.Records[0]
	if record.Message == string(record.MessageID) {
		t.Fatalf("the message is still the id %q; Options.Translator is not wired", record.Message)
	}
	if record.Message != "zainstalowano" {
		t.Errorf("message = %q, want the Polish catalog entry", record.Message)
	}
}
