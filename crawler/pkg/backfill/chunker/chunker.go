package chunker

import "strings"

type Chunk struct {
	Index   int
	Content string
}

func ChunkText(text string, maxWords, overlapWords int) []Chunk {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	step := maxWords - overlapWords

	if step <= 0 {
		step = maxWords
	}

	var chunks []Chunk

	idx := 0

	for start := 0; start < len(words); start += step {
		end := min(start+maxWords, len(words))
		chunks = append(chunks, Chunk{
			idx, strings.Join(words[start:end], " "),
		})
		idx++
		if end == len(words) {
			break
		}
	}

	return chunks
}
