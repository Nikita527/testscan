package rules

import (
	"context"
	"time"

	"github.com/Nikita527/testscan/internal/parse"
	"github.com/Nikita527/testscan/scan"
)

// astModel берёт кэш из scan.Run (один spawn на файл) либо парсит сам (Check вне Run).
func astModel(file scan.File) (parse.Model, error) {
	if file.ModelOK {
		return file.Model, file.ModelErr
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return parse.File(ctx, file.Path, file.Content)
}
