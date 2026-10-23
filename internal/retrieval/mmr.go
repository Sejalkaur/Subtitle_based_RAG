package retrieval

import (
	"math"

	"subtitle-rag/internal/domain"
)

// CosineSimilarity computes cosine similarity between two float32 vectors.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, na, nb float64
	for i := range a {
		va := float64(a[i])
		vb := float64(b[i])
		dot += va * vb
		na += va * va
		nb += vb * vb
	}

	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// MMR applies Maximal Marginal Relevance diversification matching Node.js mmr.
func MMR(candidates []domain.SearchHit, queryVec []float32, k int, lambda float64) []domain.SearchHit {
	if len(candidates) == 0 || k <= 0 {
		return nil
	}

	// Filter out any candidates without vectors
	var valid []domain.SearchHit
	for _, c := range candidates {
		if len(c.Vector) > 0 {
			valid = append(valid, c)
		}
	}
	if len(valid) == 0 {
		return nil
	}

	cand := make([]domain.SearchHit, len(valid))
	copy(cand, valid)

	var picked []domain.SearchHit
	for len(picked) < k && len(cand) > 0 {
		bestIdx := 0
		bestScore := math.Inf(-1)

		for i := range cand {
			rel := CosineSimilarity(queryVec, cand[i].Vector)

			div := float64(0)
			if len(picked) > 0 {
				maxSim := math.Inf(-1)
				for _, p := range picked {
					s := CosineSimilarity(cand[i].Vector, p.Vector)
					if s > maxSim {
						maxSim = s
					}
				}
				div = maxSim
			}

			score := lambda*rel - (1.0-lambda)*div
			if score > bestScore {
				bestScore = score
				bestIdx = i
			}
		}

		picked = append(picked, cand[bestIdx])
		cand = append(cand[:bestIdx], cand[bestIdx+1:]...)
	}

	return picked
}
