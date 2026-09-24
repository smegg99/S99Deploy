// internal/cli/locale.go

package cli

import (
	"errors"
	"strings"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/spf13/cobra"

	"github.com/smegg99/s99deploy/internal/messages"
)

// flagFromArgs finds a long flag in the arguments, in both spellings.
func flagFromArgs(args []string, name string) (string, bool) {
	// Anything after the -- terminator is ignored, and the last one wins, like
	// every other flag.
	value, found := "", false
	for i, arg := range args {
		switch {
		case arg == "--":
			return value, found
		case arg == name:
			if i+1 < len(args) {
				value, found = args[i+1], true
			}
		case strings.HasPrefix(arg, name+"="):
			value, found = strings.TrimPrefix(arg, name+"="), true
		}
	}
	return value, found
}

// langFromArgs is flagFromArgs for the flag that is read before the tree exists.
func langFromArgs(args []string) (string, bool) { return flagFromArgs(args, "--lang") }

// boolFromArgs reports whether a boolean long flag was given and not switched off.
func boolFromArgs(args []string, name string) bool {
	// The command tree does not exist yet to parse it properly, and --verbose is
	// a boolean: flagFromArgs takes the next argument as a value, so it would
	// read `--verbose false` as verbose.
	on := false
	for _, arg := range args {
		switch {
		case arg == "--":
			return on
		case arg == name:
			on = true
		case strings.HasPrefix(arg, name+"="):
			on = arg[len(name)+1:] != "false" && arg[len(name)+1:] != "0"
		}
	}
	return on
}

// languageFrom resolves the language before the tree is built.
func languageFrom(args []string) (string, error) {
	// Short, Long and every flag description are set at construction, and help
	// renders them without running a hook. An unsupported explicit choice is a
	// usage error.
	chosen, ok := langFromArgs(args)
	if !ok {
		return messages.LanguageFromEnv(), nil
	}
	if !messages.Has(chosen) {
		words := messages.Localizer(messages.LanguageFromEnv())
		return messages.LanguageFromEnv(), errors.New(messages.CliCommandInvalidLanguage(
			words, messages.CliCommandInvalidLanguageParams{
				Value:     chosen,
				Supported: strings.Join(messages.Locales, ", "),
			}))
	}
	return chosen, nil
}

// localize writes the catalog's sentences over the tree cobra built.
func localize(root *cobra.Command, words *i18n.Localizer) {
	root.InitDefaultCompletionCmd()
	root.InitDefaultHelpCmd()

	root.Short = messages.CliCommandRootShort(words)
	root.Long = messages.CliCommandRootLong(words)

	flags := root.PersistentFlags()
	flags.Lookup("lang").Usage = messages.CliCommandLangFlag(words)
	flags.Lookup("color").Usage = messages.CliCommandColorFlag(words)
	flags.Lookup("verbose").Usage = messages.CliCommandVerboseFlag(words)

	// Keep cobra's layout and flag rendering; translate its labels.
	root.SetUsageTemplate(strings.NewReplacer(
		"Usage:", messages.CliCommandUsage(words),
		"Available Commands:", messages.CliCommandCommands(words),
		"Global Flags:", messages.CliCommandGlobalFlags(words),
		"Flags:", messages.CliCommandFlags(words),
		`Use "{{.CommandPath}} [command] --help" for more information about a command.`,
		messages.CliCommandHelpHint(words, messages.CliCommandHelpHintParams{
			Command: "{{.CommandPath}} [command] --help",
		}),
	).Replace(root.UsageTemplate()))

	var visit func(*cobra.Command)
	visit = func(cmd *cobra.Command) {
		switch cmd.Name() {
		case "install":
			cmd.Short = messages.CliCommandInstallShort(words)
		case "up":
			cmd.Short = messages.CliCommandUpShort(words)
			cmd.Flags().Lookup("timeout").Usage = messages.CliCommandTimeoutFlag(words)
		case "run":
			cmd.Short = messages.CliCommandRunShort(words)
		case "uninstall":
			cmd.Short = messages.CliCommandUninstallShort(words)
			cmd.Flags().Lookup("purge").Usage = messages.CliCommandPurgeFlag(words)
			cmd.Flags().Lookup("yes").Usage = messages.CliCommandYesFlag(words)
		case "help":
			cmd.Short = messages.CliCommandHelpShort(words)
			cmd.Long = cmd.Short
		case "completion":
			cmd.Short = messages.CliCommandCompletionShort(words)
			cmd.Long = cmd.Short
		case "bash", "zsh", "fish", "powershell":
			cmd.Short = messages.CliCommandShellShort(words, messages.CliCommandShellShortParams{Shell: cmd.Name()})
			cmd.Long = cmd.Short
		}

		cmd.InitDefaultHelpFlag()
		cmd.InitDefaultVersionFlag()
		cmd.Flags().Lookup("help").Usage = messages.CliCommandHelpFlag(words)
		if flag := cmd.Flags().Lookup("version"); flag != nil {
			flag.Usage = messages.CliCommandVersionFlag(words)
		}
		for _, child := range cmd.Commands() {
			visit(child)
		}
	}
	visit(root)
}
