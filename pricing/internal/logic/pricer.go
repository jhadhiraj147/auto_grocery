package logic

import "math"

// CalculatePrice derives selling price from cost and stock-based margin adjustments.
func CalculatePrice(costPrice float64, currentStock int) float64 {
	margin := 1.20

	if currentStock > 0 && currentStock < 5 {
		margin += 0.15
	}

	if currentStock > 100 {
		margin -= 0.10
	}

	newPrice := costPrice * margin

	return math.Round(newPrice*100) / 100
}
