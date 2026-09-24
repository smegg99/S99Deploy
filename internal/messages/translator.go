// internal/messages/translator.go

package messages

import (
	"github.com/smegg99/s99logger"
	logi18n "github.com/smegg99/s99logger/i18n"
)

// Translator renders a log event id as the sentence its catalog carries.
func Translator() s99logger.Translator {
	// logi18n.New registers JSON itself, so Options carries no Decoders entry
	// even though the shipped example passes one. A nil Translator leaves the
	// id in the record, which is visibly wrong rather than blank.
	built, err := logi18n.New(logi18n.Options{
		FS: Files, Files: Names, DefaultLanguage: DefaultLocale,
	})
	if err != nil {
		return nil
	}
	return built
}
