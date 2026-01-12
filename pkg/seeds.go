package frontier

import (
	"bufio"
	"errors"
	"log/slog"
	"os"

	"github.com/devraulu/crowlr/pkg/process"
)

var (
	ErrNoSeeds = errors.New("no seeds loaded")
)

func LoadSeeds(path string, f *Frontier) error {
	slog.Info("loading seeds", "path", path)
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		url := scanner.Text()
		normalized, err := process.Normalize(url)
		if err != nil {
			slog.Error("couldn't normalize seed", slog.String("seed", url), slog.Any("err", err))
			continue
		}
		f.Push(normalized, url, "")
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	if f.Len() == 0 {
		return ErrNoSeeds
	}

	slog.Info("loaded seeds", "count", f.Len())
	return nil
}
