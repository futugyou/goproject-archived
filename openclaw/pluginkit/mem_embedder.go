package pluginkit

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"regexp"
	"strings"
)

type HashingEmbedder struct {
	Dimensions int
}

var tokenRe = regexp.MustCompile(`[a-zA-Z0-9]+`)

func (h *HashingEmbedder) Embed(input string) []float32 {
	vector := make([]float32, h.Dimensions)
	tokens := tokenRe.FindAllString(strings.ToLower(input), -1)

	for _, token := range tokens {
		hash := sha256.Sum256([]byte(token))
		index := binary.LittleEndian.Uint32(hash[0:4]) % uint32(h.Dimensions)
		sign := float32(1.0)
		if (hash[4] & 1) != 0 {
			sign = -1.0
		}
		vector[index] += sign
	}

	var sumSq float64
	for _, v := range vector {
		sumSq += float64(v * v)
	}
	norm := math.Sqrt(sumSq)

	if norm == 0 {
		vector[0] = 1.0
	} else {
		for i := range vector {
			vector[i] = float32(float64(vector[i]) / norm)
		}
	}
	return vector
}
